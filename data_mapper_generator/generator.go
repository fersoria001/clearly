package data_mapper_generator

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"golang.org/x/tools/imports"
)

const configFileName = "config.json"
const dataMapperPkg = "data_mapper"
const generatedPkgName = "generated"
const generatedTestPkgName = "generated_tests"

var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

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
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	Table           string            `json:"table"`
	Fields          []FieldType       `json:"fields"`
	Pkg             string            `json:"pkg"`
	Dir             string            `json:"dir"`
	Builder         string            `json:"builder"`
	ValidatedFields []*ValidatedField `json:"-"`
}

type DbConfig struct {
	Pkg     string `json:"pkg"`
	Dir     string `json:"dir"`
	Builder string `json:"builder"`
}

type TestConfig struct {
	Db DbConfig `json:"db"`
}

type Config struct {
	Objects []ObjectType `json:"objects"`
	Tests   *TestConfig  `json:"tests"`
}

func (o Config) valid(caller string) error {
	for i := range o.Objects {
		err := o.Objects[i].valid(caller)
		if err != nil {
			return err
		}
	}
	if o.Tests != nil {
		err := o.Tests.valid(caller)
		if err != nil {
			return err
		}
	}
	return nil
}

func (o TestConfig) valid(caller string) error {
	err := o.Db.valid(caller)
	if err != nil {
		return err
	}
	return nil
}

// Checks if the dir exists.
// Parses the dir into an ast.
// Inspect the ast and determine if the pkg and builder method exists.
// and the builder method has a the correct signature
// the expected signature is func() (*pgpool.Pool, error)
// where pgpool is imported from jackc/pgx/v5.
func (o DbConfig) valid(caller string) error {
	if o.Pkg == "" {
		return fmt.Errorf("db config obj is present db pkg name is required")
	}
	if o.Dir == "" {
		return fmt.Errorf("db config obj is present db pkg dir is required")
	}
	if o.Builder == "" {
		return fmt.Errorf("db obj is present builder method name is required")
	}
	filePath := filepath.Join(caller, o.Dir)
	_, files, info, err := pkgFiles(filePath, o.Pkg)
	if err != nil {
		return fmt.Errorf("dbConfig validation error: %w", err)
	}
	checkReturn := func(tuple *types.Tuple) bool {
		if tuple.Len() != 2 {
			return false
		}
		pool := tuple.At(0)
		err := tuple.At(1)
		if pool.Type().String() == "*github.com/jackc/pgx/v5/pgxpool.Pool" && err.Type().String() == "error" {
			return true
		}
		return false
	}
	checkSig := func(sig *types.Signature) bool {
		if sig.Params().Len() != 0 {
			return false
		}
		return checkReturn(sig.Results())
	}
	isCorrect := false
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if n.Name.Name == o.Builder {
					isCorrect = checkSig(info.Defs[n.Name].Type().(*types.Signature))
				}
			}
			return true
		})
	}
	if !isCorrect {
		return fmt.Errorf("the builder method %s found in package %s has an incorrect signature", o.Builder, o.Pkg)
	}
	return nil
}

// Parses the dir, look for the target package if found type check it
// and return the pkg, the pkg files, type.Info with defs and types and nil
// or an error if the target package is not found
func pkgFiles(dir string, targetPkg string) (*types.Package, []*ast.File, *types.Info, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(
		fset,
		dir,
		func(fi fs.FileInfo) bool {
			return fi.Name() != ""
		},
		parser.SkipObjectResolution,
	)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("\n dbConfig dir:\n %s \n is not a valid go source file: %w", dir, err)
	}
	conf := types.Config{
		Importer: importer.ForCompiler(fset, "source", nil),
	}
	info := &types.Info{
		Defs:  make(map[*ast.Ident]types.Object),
		Types: make(map[ast.Expr]types.TypeAndValue),
	}
	if pkg, ok := pkgs[targetPkg]; ok {
		files := slices.Collect(maps.Values(pkg.Files))
		typesPkg, err := conf.Check(dir, fset, files, info)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("type checking err in %s:\n\t %w", dir, err)
		}
		return typesPkg, files, info, nil
	}
	return nil, nil, nil, fmt.Errorf("targetPkg %s not found in %s", targetPkg, dir)
}

type ValidatedField struct {
	name       string
	dataType   string
	haveGetter bool
	getterName string
}

// Check for nil fields in the object type, all fields with the exception of ValidatedFields are required.
// Parses the caller/objectType.Dir, look for the package with name equal to objectType.Pkg.
// Inside the package, find the objectType.Name,it must be a concrete struct type with all
// the specified unexported fields.
// Analyze the methods set of the struct, it has to contain one method for each
// field that takes no parameters, returns only the field data type and is
// named after the field with the first letter capital cased.
// Searchs for a function with name equal objectType.Builder , it has to
// accept as many parameters as specified fields typed in the same order as the
// objectType.Fields slice (configuration file) and to return a single value
// with its type equal to (objectType.Pkg).(objectType.Name).
// If all this requirements are met, ObjectType is considered valid and the return is nil,
// otherwise it returns an error.
func (o *ObjectType) valid(caller string) error {
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
	if o.Pkg == "" {
		return fmt.Errorf("the pkg name is required")
	}
	if o.Dir == "" {
		return fmt.Errorf("the domain object file name is required")
	}
	if o.Builder == "" {
		return fmt.Errorf("the domain object builder method name is required")
	}
	pkg, files, info, err := pkgFiles(filepath.Join(caller, o.Pkg), o.Pkg)
	if err != nil {
		return err
	}
	obj := pkg.Scope().Lookup(o.Name)
	if obj == nil {
		return fmt.Errorf("could not find the object %s in %s", o.Name, pkg.Path())
	}

	expectedFields := make(map[string]*ValidatedField, 0)
	for _, v := range o.Fields {
		expectedFields[v.Name] = &ValidatedField{haveGetter: false}
	}
	if ctype, ok := obj.Type().Underlying().(*types.Struct); ok {
		for i := range ctype.NumFields() {
			v := ctype.Field(i)
			if efield, ok := expectedFields[v.Name()]; ok {
				efield.name = v.Name()
				efield.dataType = v.Type().String()
			}
		}
	}
	mset := types.NewMethodSet(obj.Type())
	checkReturn := func(tuple *types.Tuple, expected *ValidatedField) bool {
		if tuple.Len() != 1 {
			return false
		}
		rType := tuple.At(0).Type()
		return rType.String() == expected.dataType
	}
	for i := range mset.Len() {
		meth := mset.At(i).Obj()
		if efield, ok := expectedFields[strings.ToLower(meth.Name())]; ok && meth.Exported() {
			if sig, ok := meth.Type().(*types.Signature); ok {
				efield.haveGetter = checkReturn(sig.Results(), efield)
				efield.getterName = meth.Name()
			}
		}
	}
	err = nil
	for k, v := range expectedFields {
		if !v.haveGetter {
			if err != nil {
				err = fmt.Errorf("%w\nthe field %s from type %s does not have a getter method of type func () %s",
					err, k, o.Name, v.dataType)
			} else {
				err = fmt.Errorf("\nthe field %s from type %s does not have a getter method of type func () %s",
					k, o.Name, v.dataType)
			}
		}
	}
	if err != nil {
		return err
	}
	expectedParams := slices.Collect(maps.Values(expectedFields))
	checkSig := func(sig *types.Signature) bool {
		params := sig.Params()
		if params.Len() != len(o.Fields) {
			return false
		}
		for i := range params.Len() {
			param := params.At(i)
			expectedParam := expectedParams[i]
			if param.Type().String() != expectedParam.dataType {
				return false
			}
		}
		result := sig.Results()
		if result.Len() != 1 {
			return false
		}
		if types.TypeString(result.At(0).Type(), (*types.Package).Name) != fmt.Sprintf("*%s.%s", o.Pkg, o.Name) {
			return false
		}
		return true
	}
	isCorrect := false
	for _, file := range files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if n.Name.Name == o.Builder {
					isCorrect = checkSig(info.Defs[n.Name].Type().(*types.Signature))
				}
			}
			return true
		})
	}
	if !isCorrect {
		return fmt.Errorf("the builder method %s for type %s found in package %s has an incorrect signature",
			o.Builder, o.Name, o.Pkg)
	}
	o.ValidatedFields = expectedParams
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
	buff   *bytes.Buffer
}

func (g DataMapperGenerator) WithTests() bool {
	return g.config.Tests != nil
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
		buff:   bytes.NewBuffer(make([]byte, 0)),
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

func (g *DataMapperGenerator) wln(w string) {
	_, err := g.buff.WriteString(fmt.Sprintf("%s\n", w))
	if err != nil {
		panic(err)
	}
}

func (g *DataMapperGenerator) generateImports(o ObjectType) {
	requiredImports := []string{
		"fmt",
		"github.com/jackc/pgx/v5",
		"github.com/jackc/pgx/v5/pgxpool",
		"clearly-not-a-secret-project/data_mapper",
	}
	objPkgPath := fmt.Sprintf("%s/%s", filepath.Base(g.caller), o.Pkg)
	allImports := make([]string, 0)
	allImports = append(allImports, objPkgPath)
	allImports = append(allImports, requiredImports...)
	g.wln("import (")
	for _, v := range allImports {
		g.wln(fmt.Sprintf("\"%s\"", v))
	}
	g.wln(")")
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

// Creates the folder for the package in caller/pkg
// if err it panics else writes package name to the generator buffer
// returns the absolute to the current project path to the pkg.
func (g *DataMapperGenerator) generateNewPkg(pkgName string) string {
	generatedDir := filepath.Join(g.caller, pkgName)
	err := os.MkdirAll(generatedDir, os.ModePerm)
	if err != nil {
		panic(err)
	}
	g.wln(fmt.Sprintf("package %s", pkgName))
	return generatedDir
}

func (g *DataMapperGenerator) generateDataMapperStructType(o ObjectType) {
	index := -1
	for i := range o.ValidatedFields {
		if o.ValidatedFields[i].name == "id" {
			index = i
		}
	}
	if index < 0 {
		panic(fmt.Errorf("could not find id field in the validated fields"))
	}
	idField := o.ValidatedFields[index]
	g.wln(fmt.Sprintf("type %sDataMapper struct {", o.Name))
	g.wln(fmt.Sprintf("%s.PostgreSQLDataMapper[%s.DomainObject[%s],%s]",
		dataMapperPkg, dataMapperPkg, idField.dataType, idField.dataType,
	))
	g.wln("}")
}

func (g *DataMapperGenerator) generateDataMapperCBuilder(o ObjectType) {
	index := -1
	for i := range o.ValidatedFields {
		if o.ValidatedFields[i].name == "id" {
			index = i
		}
	}
	if index < 0 {
		panic(fmt.Errorf("could not find id field in the validated fields"))
	}
	idField := o.ValidatedFields[index]
	g.wln(fmt.Sprintf(
		`func New%sDataMapper(pool *pgxpool.Pool,loadedMap map[%s]%s.DomainObject[%s],) (%sDataMapper, error) {`,
		o.Name, idField.dataType, dataMapperPkg, idField.dataType, o.Name,
	))
	g.wln(fmt.Sprintf(
		"return %sDataMapper{",
		o.Name,
	))
	g.wln(fmt.Sprintf(
		"PostgreSQLDataMapper: %s.PostgreSQLDataMapper[%s.DomainObject[%s],%s]{",
		dataMapperPkg, dataMapperPkg, idField.dataType, idField.dataType,
	))
	g.wln("Db: pool,")
	g.wln("LoadedMap: loadedMap,")
	g.wln(fmt.Sprintf("FindStatement: \"%s\",", g.findStmt(o)))
	g.wln(fmt.Sprintf("InsertStatement: \"%s\",", g.insertStmt(o)))
	g.wln(fmt.Sprintf("UpdateStatement: \"%s\",", g.updateStmt(o)))
	g.wln(fmt.Sprintf("RemoveStatement: \"%s\",", g.removeStmt(o)))
	g.generateDoLoadFn(o)
	g.generateDoInsertFn(o)
	g.generateDoUpdateFn(o)
	g.wln("},")
	g.wln("},nil")
	g.wln("}")
}

func (g *DataMapperGenerator) generateDoUpdateFn(o ObjectType) {
	index := -1
	getters := make([]string, 0)
	for i := range o.ValidatedFields {
		if o.ValidatedFields[i].name == "id" {
			index = i
		}
		getters = append(getters, o.ValidatedFields[i].getterName)
	}
	if index < 0 {
		panic(fmt.Errorf("could not find id field in the validated fields"))
	}
	idField := o.ValidatedFields[index]
	g.wln(fmt.Sprintf(
		"DoUpdate: func(obj %s.DomainObject[%s], stmt *%s.PreparedStatement) error {",
		dataMapperPkg, idField.dataType, dataMapperPkg,
	))
	g.wln(fmt.Sprintf(
		"subject, ok := obj.(*%s.%s)", o.Pkg, o.Name,
	))
	g.wln("if !ok { return fmt.Errorf(\"wrong type assertion\")}")
	for i := range getters {
		g.wln(fmt.Sprintf("stmt.Append(subject.%s())", getters[i]))
	}
	g.wln("return nil },")
}

func (g *DataMapperGenerator) generateDoInsertFn(o ObjectType) {
	index := -1
	getters := make([]string, 0)
	for i := range o.ValidatedFields {
		if o.ValidatedFields[i].name == "id" {
			index = i
		}
		getters = append(getters, o.ValidatedFields[i].getterName)
	}
	if index < 0 {
		panic(fmt.Errorf("could not find id field in the validated fields"))
	}
	idField := o.ValidatedFields[index]
	g.wln(fmt.Sprintf(
		"DoInsert: func(obj %s.DomainObject[%s], stmt *%s.PreparedStatement) error {",
		dataMapperPkg, idField.dataType, dataMapperPkg,
	))
	g.wln(fmt.Sprintf(
		"subject, ok := obj.(*%s.%s)",
		o.Pkg, o.Name,
	))
	g.wln("if !ok { return fmt.Errorf(\"wrong type assertion \") }")
	for i := range getters {
		g.wln(fmt.Sprintf("stmt.Append(subject.%s())", getters[i]))
	}
	g.wln("return nil },")
}

func (g *DataMapperGenerator) generateDoLoadFn(o ObjectType) {
	index := -1
	variables := make(map[string]string, 0)
	for i := range o.ValidatedFields {
		if o.ValidatedFields[i].name == "id" {
			index = i
		}
		variables[o.ValidatedFields[i].name] = o.ValidatedFields[i].dataType
	}
	if index < 0 {
		panic(fmt.Errorf("could not find id field in the validated fields"))
	}
	idField := o.ValidatedFields[index]
	g.wln(fmt.Sprintf(
		"DoLoad: func (resultSet pgx.Rows) (%s.DomainObject[%s],error){",
		dataMapperPkg, idField.dataType,
	))
	g.wln("var (")
	for k, v := range variables {
		g.wln(fmt.Sprintf("%s %s", k, v))
	}
	g.wln(")")
	g.wln("err := resultSet.Scan(")
	for k := range variables {
		g.wln(fmt.Sprintf("&%s,", k))
	}
	g.wln(")")
	g.wln("if err != nil {")
	g.wln("return nil, err")
	g.wln("}")
	g.wln(fmt.Sprintf("return %s.%s(", o.Pkg, o.Builder))
	for k := range variables {
		g.wln(fmt.Sprintf("%s,", k))
	}
	g.wln("), nil")
	g.wln("},")
}

func (g *DataMapperGenerator) generate(o ObjectType) error {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}
	}()
	newPkgPath := g.generateNewPkg(generatedPkgName)
	g.generateImports(o)
	g.generateDataMapperStructType(o)
	g.generateDataMapperCBuilder(o)

	snake := matchFirstCap.ReplaceAllString(o.Name, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	newFileName := filepath.Join(newPkgPath, fmt.Sprintf("%s_data_mapper.go", strings.ToLower(snake)))
	formatted, err := imports.Process(newFileName, g.buff.Bytes(), &imports.Options{Comments: true})
	if err != nil {
		return fmt.Errorf("error at formatting imports %w", err)
	}
	err = os.WriteFile(newFileName, formatted, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

func (g *DataMapperGenerator) generateTestImports(o ObjectType) {
	requiredImports := []string{
		"clearly-not-a-secret-project/data_mapper",
		"testing",
		"context",
	}
	dbPkg := fmt.Sprintf("%s/%s", filepath.Base(g.caller), g.config.Tests.Db.Pkg)
	objPkg := fmt.Sprintf("%s/%s", filepath.Base(g.caller), o.Pkg)
	generatedPkg := fmt.Sprintf("%s/%s", filepath.Base(g.caller), generatedPkgName)
	allImports := make([]string, 0)
	allImports = append(allImports, objPkg)
	if dbPkg != objPkg {
		allImports = append(allImports, dbPkg)
	}
	allImports = append(allImports, generatedPkg)
	allImports = append(allImports, requiredImports...)
	g.wln("import (")
	for _, v := range allImports {
		g.wln(fmt.Sprintf("\"%s\"", v))
	}
	g.wln(")")
}

func (g *DataMapperGenerator) generateTestInsertFunc(o ObjectType) {
	g.wln("t.Run(\"Insert\", func(t *testing.T) {")
	g.wln("for _, v := range testData {")
	g.wln(fmt.Sprintf(
		"aggregate := %s.%s(",
		o.Pkg, o.Builder,
	))
	for _, v := range o.ValidatedFields {
		g.wln(fmt.Sprintf("v.%s,", v.getterName))
	}
	g.wln(")")
	g.wln("id, err := dataMapper.Insert(ctx, aggregate)")
	g.wln("if err != nil { t.Fatal(err) }")
	g.wln(fmt.Sprintf(
		"if id != aggregate.Id() { t.Fatal(AssertionError{name: \"id\", expected:aggregate.%s(), found:id}.Error())}",
		"Id"),
	)
	g.wln("}})")
}

func (g *DataMapperGenerator) generateTestFindFunc(o ObjectType) {
	g.wln("t.Run(\"Find\", func(t *testing.T) {")
	g.wln("for _, v := range testData {")
	g.wln("dbAggregate,err := dataMapper.Find(ctx, v.Id)")
	g.wln("if err != nil { t.Fatal(err) }")
	g.wln("if dbAggregate == nil { t.Fatal(\"the returned object is nil\") }")
	g.wln(fmt.Sprintf(
		"aggregate, ok := dbAggregate.(*%s.%s)",
		o.Pkg, o.Name,
	))
	g.wln("if !ok { t.Fatal(\"wrong type assertion\") }")
	for _, v := range o.ValidatedFields {
		g.wln(fmt.Sprintf(
			`if aggregate.%s() != v.%s {
		t.Fatal(AssertionError{name: "%s", expected:v.%s, found:aggregate.%s()}.Error())
			}`,
			v.getterName, v.getterName, v.name, v.getterName, v.getterName,
		))
	}
	g.wln("}})")
}

func (g *DataMapperGenerator) generateTestRemoveFunc(o ObjectType) {
	g.wln("t.Run(\"Remove\", func(t *testing.T) {")
	g.wln("for _,v := range testData {")
	g.wln("err := dataMapper.Remove(ctx, v.Id)")
	g.wln("if err != nil { t.Fatal(err) }")
	g.wln("}})")
}

func (g *DataMapperGenerator) generateTestFn(o ObjectType) {
	index := -1
	variables := make(map[string]string, 0)
	for i := range o.ValidatedFields {
		if o.ValidatedFields[i].name == "id" {
			index = i
		}
		variables[o.ValidatedFields[i].name] = o.ValidatedFields[i].dataType
	}
	if index < 0 {
		panic(fmt.Errorf("could not find id field in the validated fields"))
	}
	idField := o.ValidatedFields[index]
	g.wln(fmt.Sprintf(
		"func Test%sDataMapper(t *testing.T) {",
		o.Name,
	))
	g.wln("ctx := context.Background()")
	g.wln(fmt.Sprintf("pool, err := %s.%s()", g.config.Tests.Db.Pkg, g.config.Tests.Db.Builder))
	g.wln("if err != nil { t.Fatal(err) }")
	g.wln(fmt.Sprintf(
		"loadedMap := make(map[%s]%s.DomainObject[%s],0)",
		idField.dataType, dataMapperPkg, idField.dataType,
	))
	g.wln(fmt.Sprintf(
		"dataMapper, err := %s.New%sDataMapper(pool, loadedMap)",
		generatedPkgName, o.Name,
	))
	g.wln("if err != nil { t.Fatal(err) }")
	g.generateTestInsertFunc(o)
	g.generateTestFindFunc(o)
	g.generateTestRemoveFunc(o)
	g.wln("}")
}

func (g *DataMapperGenerator) generateTestData(o ObjectType) {
	g.wln("var testData = map[string] struct{")
	for _, v := range o.ValidatedFields {
		g.wln(fmt.Sprintf("%s %s", v.getterName, v.dataType))
	}
	g.wln("}{")
	g.wln("\"valid\": {")
	for _, v := range o.ValidatedFields {
		var randomValue any
		switch v.dataType {
		case "string":
			randomValue = fmt.Sprintf("\"%s\"", randStringBytesMaskImprSrcUnsafe(10))
		case "int":
			randomValue = randInt()
		}
		g.wln(fmt.Sprintf(
			"%s: %v,",
			v.getterName, randomValue,
		))
	}
	g.wln("},}")
}

func (g *DataMapperGenerator) GenerateAllTests() error {
	if g.config.Tests == nil {
		return fmt.Errorf("the test config is not defined")
	}
	err := g.generateTestErrorsFile()
	if err != nil {
		return err
	}
	for i := range g.config.Objects {
		err := g.generateTest(g.config.Objects[i])
		if err != nil {
			return err
		}
	}
	return nil
}

func (g *DataMapperGenerator) generateAssertionErrorType() {
	g.wln("type AssertionError struct {")
	g.wln("name string")
	g.wln("expected string")
	g.wln("found string")
	g.wln("}")
	g.wln("func (e AssertionError) Error() string {")
	g.wln("return fmt.Errorf(\"err field %s: expectd %s found %s\",e.name,e.expected,e.found).Error()")
	g.wln("}")
}

func (g *DataMapperGenerator) generateTestErrorsFile() error {
	g.buff.Reset()
	newPkgPath := g.generateNewPkg(generatedTestPkgName)
	g.wln("import (")
	g.wln("\"fmt\"")
	g.wln(")")
	g.generateAssertionErrorType()
	newFileName := filepath.Join(newPkgPath, "errors.go")
	formatted, err := imports.Process(newFileName, g.buff.Bytes(), &imports.Options{Comments: true})
	if err != nil {
		return fmt.Errorf("error at formatting imports %w", err)
	}
	err = os.WriteFile(newFileName, formatted, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}

func (g *DataMapperGenerator) generateTest(o ObjectType) error {
	g.buff.Reset()
	newPkgPath := g.generateNewPkg(generatedTestPkgName)
	g.generateTestImports(o)
	g.generateTestData(o)
	g.generateTestFn(o)
	snake := matchFirstCap.ReplaceAllString(o.Name, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	newFileName := filepath.Join(newPkgPath, fmt.Sprintf("%s_data_mapper_test.go", strings.ToLower(snake)))
	formatted, err := imports.Process(newFileName, g.buff.Bytes(), &imports.Options{Comments: true})
	if err != nil {
		return fmt.Errorf("error at formatting imports %w", err)
	}
	err = os.WriteFile(newFileName, formatted, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}
