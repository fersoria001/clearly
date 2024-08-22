package data_mapper_generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	mapset "github.com/deckarep/golang-set"
)

const configFileName = "config.json"

type DomainObjectType int

const (
	AGGREGATE DomainObjectType = iota
	ENTITY
	VALUEOBJECT
)

func (t DomainObjectType) String() string {
	switch t {
	case AGGREGATE:
		return "aggregate"
	case ENTITY:
		return "entity"
	case VALUEOBJECT:
		return "valueObject"
	default:
		return ""
	}
}

func ParseDomainObjectType(v string) (DomainObjectType, error) {
	switch v {
	case "aggregate":
		return AGGREGATE, nil
	case "entity":
		return ENTITY, nil
	case "valueObject":
		return VALUEOBJECT, nil
	default:
		return -1, fmt.Errorf("exhaustive check: domain object type is invalid")
	}
}

type FieldType struct {
	Name   string `json:"name"`
	Column string `json:"column"`
}

type ObjectType struct {
	Caller  string      `json:"caller"`
	Name    string      `json:"name"`
	Type    string      `json:"type"`
	Table   string      `json:"table"`
	Fields  []FieldType `json:"fields"`
	Pkg     string      `json:"pkg"`
	Dir     string      `json:"dir"`
	Builder string      `json:"builder"`
}

type Config struct {
	Objects []ObjectType `json:"objects"`
}

func (o Config) valid(caller string) error {
	for i := range o.Objects {
		err := o.Objects[i].valid(caller)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o ObjectType) valid(caller string) error {
	if o.Name == "" {
		return fmt.Errorf("the domain object name is required")
	}
	if o.Type == "" {
		return fmt.Errorf("the domain object type is required")
	}
	_, err := ParseDomainObjectType(o.Type)
	if err != nil {
		return err
	}
	if o.Table == "" {
		return fmt.Errorf("the domain object table name is required")
	}
	if o.Fields == nil || len(o.Fields) < 1 {
		return fmt.Errorf("the object fields are required")
	}
	for i := range o.Fields {
		err := o.Fields[i].valid()
		if err != nil {
			return nil
		}
	}
	if o.Pkg == "" {
		return fmt.Errorf("the pkg name is required")
	}
	if o.Dir == "" {
		return fmt.Errorf("the domain object file name is required")
	}
	path := filepath.Join(caller, o.Dir)
	_, err = os.Stat(path)
	if err != nil {
		return err
	}
	if o.Builder == "" {
		return fmt.Errorf("the domain object builder method name is required")
	}
	return nil
}

func (o ObjectType) validateGetters(funcDecls []*ast.FuncDecl) error {
	expectedFields := map[string]bool{}
	for _, v := range o.Fields {
		expectedFields[v.Name] = false
	}
	for _, v := range funcDecls {
		name := strings.ToLower(v.Name.Name)
		if _, ok := expectedFields[name]; ok {
			expectedFields[name] = true
		}
	}
	var err error
	for k, v := range expectedFields {
		if !v {
			if err != nil {
				err = fmt.Errorf("%w \nthe object %s does not have a public getter for %s", err, o.Name, k)
			} else {
				err = fmt.Errorf("\nthe object %s does not have a public getter for %s", o.Name, k)
			}
		}
	}
	if err != nil {
		return err
	}
	return nil
}

func (o ObjectType) validateAstFields(fields []*ast.Field) error {
	expectedFields := map[string]bool{}
	for _, v := range o.Fields {
		expectedFields[v.Name] = false
	}
	for _, v := range fields {
		name := v.Names[0].Name
		if _, ok := expectedFields[name]; ok {
			expectedFields[name] = true
		}
	}
	var err error
	for k, v := range expectedFields {
		if !v {
			if err != nil {
				err = fmt.Errorf("%w \nthe object %s does not have the field %s", err, o.Name, k)
			} else {
				err = fmt.Errorf("\nthe object %s does not have the field %s", o.Name, k)
			}
		}
	}
	if err != nil {
		return err
	}
	return nil
}

func (o FieldType) valid() error {
	if o.Name == "" {
		return fmt.Errorf("the fieldType name is required")
	}
	if o.Column == "" {
		return fmt.Errorf("the fieldType column name is required")
	}
	return nil
}

func readConfig(caller string) (*Config, error) {
	data, err := os.ReadFile(filepath.Join(caller, configFileName))
	if err != nil {
		return nil, fmt.Errorf("error reading config file %w", err)
	}
	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	err = config.valid(caller)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

type DataMapperGenerator struct {
	caller string
	config *Config
}

func New() (*DataMapperGenerator, error) {
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		return nil, fmt.Errorf("could not obtain the dir of the caller")
	}
	caller, _ := filepath.Split(file)
	if os.Getenv("ENVIRONMENT") == "DEV" {
		projectDir, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("could not get working dir in dev mode %w", err)
		}
		caller = filepath.Dir(projectDir)
	}
	config, err := readConfig(caller)
	if err != nil {
		return nil, err
	}
	return &DataMapperGenerator{
		caller: caller,
		config: config,
	}, nil
}

func (g *DataMapperGenerator) GenerateAll() error {
	for i := range g.config.Objects {
		err := g.generate(g.config.Objects[i])
		if err != nil {
			return err
		}
	}
	return nil
}

type FieldsMap map[string]string

func (g DataMapperGenerator) imports(domainPkg string) []string {
	requiredImports := []string{
		"fmt",
		"github.com/jackc/pgx/v5",
		"github.com/jackc/pgx/v5/pgxpool",
		"clearly-not-a-secret-project/data_mapper",
	}
	allImports := make([]string, 0)
	allImports = append(allImports, domainPkg)
	allImports = append(allImports, requiredImports...)
	return allImports
}

func (g DataMapperGenerator) fieldsMap(fields []*ast.Field) FieldsMap {
	fieldsMap := map[string]string{}
	for i := range fields {
		if len(fields[i].Names) == 1 {
			name := fields[i].Names[0].Name
			fieldsMap[name] = fmt.Sprintf("%v", fields[i].Type)
		}
	}
	return fieldsMap
}

func (g DataMapperGenerator) getters(methods []*ast.FuncDecl) []string {
	getters := make([]string, 0, len(methods))
	for i := range methods {
		getters = append(getters, methods[i].Name.Name)
	}
	return getters
}

func (g *DataMapperGenerator) findStmt(o ObjectType) string {
	columns := make([]string, 0, len(o.Fields))
	for i := range o.Fields {
		columns = append(columns, o.Fields[i].Column)
	}
	columnNames := strings.Join(columns, ", ")
	stmt := fmt.Sprintf(`SELECT %v FROM %s WHERE ID = $1;`, columnNames, o.Table)
	return stmt
}

func (g *DataMapperGenerator) insertStmt(o ObjectType) string {
	columns := make([]string, 0, len(o.Fields))
	params := make([]string, 0, len(o.Fields))
	for i := range o.Fields {
		params = append(params, fmt.Sprintf("$%d", i+1))
		columns = append(columns, o.Fields[i].Column)
	}
	columnNames := strings.Join(columns, ",")
	paramNames := strings.Join(params, ",")
	stmt := fmt.Sprintf(`INSERT INTO %s (%s) VALUES (%s);`,
		o.Table, columnNames, paramNames,
	)
	return stmt
}

func (g *DataMapperGenerator) updateStmt(o ObjectType) string {
	columns := make([]string, 0, len(o.Fields))
	for i := range o.Fields {
		if o.Fields[i].Column != "id" {
			columns = append(columns,
				fmt.Sprintf("%s = $%d", o.Fields[i].Column, i+1),
			)
		}
	}
	columnNames := strings.Join(columns, ",")
	stmt := fmt.Sprintf(`UPDATE %s SET %s WHERE ID = $1`,
		o.Table, columnNames)
	return stmt
}

func (g *DataMapperGenerator) removeStmt(o ObjectType) string {
	stmt := fmt.Sprintf(`DELETE FROM %s WHERE ID = $1;`, o.Table)
	return stmt
}

func (g *DataMapperGenerator) template() string {
	templ := `
	package {{.GeneratedPackage}}

	import (
	{{- range $value := .Imports }}
		"{{ $value -}}"
	{{- end }}
	)

	type {{.StructName}}DataMapper struct {
		{{.DataMapperPackage}}.PostgreSQLDataMapper[{{.DataMapperPackage}}.DomainObject[{{.IdDataType}} ], {{.IdDataType}}]
	}

	func New{{.StructName}}DataMapper(
	pool *pgxpool.Pool,
	loadedMap map[{{.IdDataType}}]{{.DataMapperPackage}}.DomainObject[{{.IdDataType}}],
	) ({{.StructName}}DataMapper, error) {
	 	return {{.StructName}}DataMapper{
			PostgreSQLDataMapper: {{.DataMapperPackage}}.PostgreSQLDataMapper[{{.DataMapperPackage}}.DomainObject[{{.IdDataType}}], {{.IdDataType}}]{
				Db: pool,
				LoadedMap: loadedMap,
				FindStatement: "{{.FindStmt}}",
				InsertStatement: "{{.InsertStmt}}",
				UpdateStatement: "{{.UpdateStmt}}",
				RemoveStatement: "{{.RemoveStmt}}",
				DoLoad: func(resultSet pgx.Rows) ({{.DataMapperPackage}}.DomainObject[{{.IdDataType}}], error){
					var (
					{{- range $key, $value := .FieldsMap }}
   						{{ $key }} {{ $value -}}
					{{- end }}
					)
					err := resultSet.Scan(
						{{- range $key, $value := .FieldsMap }}
							&{{ $key -}},
						{{- end }}
					)
					if err != nil {
						return nil, err
					}
					return {{.Package}}.{{.Builder}}(
						{{- range $key, $value := .FieldsMap }}
							{{ $key -}},
						{{- end }}
					),nil
				},
				DoInsert: func(
				obj {{.DataMapperPackage}}.DomainObject[{{.IdDataType}}],
				stmt *{{.DataMapperPackage}}.PreparedStatement,
				) error {
					subject, ok := obj.(*{{.Package}}.{{.ObjectName}})
					if !ok {
						return fmt.Errorf("wrong type assertion")
					}
					{{- range $value := .Getters }}
					stmt.Append(subject.{{ $value }}())
					{{- end }}
					return nil
				},
				DoUpdate: func(
				obj {{.DataMapperPackage}}.DomainObject[{{.IdDataType}}],
				stmt *{{.DataMapperPackage}}.PreparedStatement,
				) error {
					subject, ok := obj.(*{{.Package}}.{{.ObjectName}})
					if !ok {
						return fmt.Errorf("wrong type assertion")
					}
					{{- range $value := .Getters }}
					stmt.Append(subject.{{ $value }}())
					{{- end }}
					return nil
				},
			},
		}, nil
	}
	`
	return templ
}

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

func toSnakeCase(str string) string {
	snake := matchFirstCap.ReplaceAllString(str, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	return strings.ToLower(snake)
}

func (g *DataMapperGenerator) generate(o ObjectType) error {
	filePath := filepath.Join(g.caller, o.Dir)
	fs := token.NewFileSet()
	file, err := parser.ParseDir(fs, filePath, nil, parser.AllErrors)
	if err != nil {
		return fmt.Errorf("error at generate parsing folder from %s into ast %w", filePath, err)
	}
	var domainPkg *ast.Package
	for i := range file {
		if file[i].Name == o.Pkg {
			domainPkg = file[i]
			break
		}
	}
	if domainPkg == nil {
		return fmt.Errorf("could not found package %s in dir %s", o.Pkg, filePath)
	}
	var structName string
	var fields []*ast.Field
	var nonPtrRecvMethods []*ast.FuncDecl
	expectedFields := mapset.NewSet()
	for i := range o.Fields {
		expectedFields.Add(o.Fields[i].Name)
	}
	for _, file := range domainPkg.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch v := n.(type) {
			case *ast.TypeSpec:
				if v.Name.Name != o.Name {
					break
				}
				structName = v.Name.Name
				if structType, ok := v.Type.(*ast.StructType); ok {
					fields = structType.Fields.List
				}
			case *ast.FuncDecl:
				if !v.Name.IsExported() {
					break
				}
				if v.Recv != nil && len(v.Recv.List) == 1 {
					if r, ok := v.Recv.List[0].Type.(*ast.Ident); ok && r.Name == o.Name &&
						expectedFields.Contains(strings.ToLower(v.Name.Name)) {
						nonPtrRecvMethods = append(nonPtrRecvMethods, v)
					}
				}
			}
			return true
		})
	}

	if structName == "" || fields == nil {
		return fmt.Errorf("struct %s not found in dir %s", o.Name, filePath)
	}

	err = o.validateAstFields(fields)
	if err != nil {
		return err
	}
	err = o.validateGetters(nonPtrRecvMethods)
	if err != nil {
		return err
	}

	out := bytes.NewBufferString("")

	t := template.Must(template.New("").Parse(g.template()))
	fieldsMap := g.fieldsMap(fields)
	pkgName := "generated"
	domainPkgDir := fmt.Sprintf("%s/%s", filepath.Base(g.caller), o.Pkg)
	imports := g.imports(domainPkgDir)

	err = t.Execute(out, map[string]interface{}{
		"Imports":           imports,
		"DataMapperPackage": "data_mapper",
		"GeneratedPackage":  pkgName,
		"Package":           o.Pkg,
		"ObjectName":        structName,
		"IdDataType":        fieldsMap["id"],
		"FindStmt":          g.findStmt(o),
		"InsertStmt":        g.insertStmt(o),
		"UpdateStmt":        g.updateStmt(o),
		"RemoveStmt":        g.removeStmt(o),
		"FieldsMap":         fieldsMap,
		"Getters":           g.getters(nonPtrRecvMethods),
		"Builder":           o.Builder,
	})
	if err != nil {
		return err
	}
	pkgDir := filepath.Join(g.caller, pkgName)
	err = os.MkdirAll(pkgDir, os.ModePerm)
	if err != nil {
		return err
	}
	newFileName := filepath.Join(pkgDir, fmt.Sprintf("%s_data_mapper.go", toSnakeCase(structName)))
	// formatted, err := imports.Process(newFileName, out.Bytes(), &imports.Options{Comments: true})
	// if err != nil {
	// 	return fmt.Errorf("error at formatting imports %w", err)
	// }
	err = os.WriteFile(newFileName, out.Bytes(), os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}
