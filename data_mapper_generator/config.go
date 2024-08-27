package data_mapper_generator

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/types"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

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
	Update bool   `json:"update"`
}

type ObjectType struct {
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	Table           string            `json:"table"`
	Fields          []FieldType       `json:"fields"`
	Pkg             string            `json:"pkg"`
	Dir             string            `json:"dir"`
	Builder         string            `json:"builder"`
	Lazy            bool              `json:"lazy"`
	ValidatedFields []*ValidatedField `json:"-"`
}

type ValidatedField struct {
	name       *string
	dataType   *string
	getterName *string
	update     bool
	setterName *string
}

type DbConfig struct {
	Pkg     string `json:"pkg"`
	Dir     string `json:"dir"`
	Builder string `json:"builder"`
}

type PkgData struct {
	pkg   *types.Package
	files []*ast.File
	info  *types.Info
}

type Config struct {
	Objects []*ObjectType `json:"objects"`
	Db      *DbConfig     `json:"db"`
	RootDir string        `json:"rootDir"`
	RootPkg string        `json:"rootPkg"`
	PkgData map[string]*PkgData
}

func (g *DataMapperGenerator) readConfig(caller string) error {
	data, err := os.ReadFile(filepath.Join(caller, configFileName))
	if err != nil {
		return fmt.Errorf("error reading config file %w", err)
	}
	var config = &Config{
		PkgData: make(map[string]*PkgData),
	}
	err = json.Unmarshal(data, config)
	if err != nil {
		return err
	}
	for _, v := range config.Objects {
		snake := matchFirstCap.ReplaceAllString(v.Name, "${1}_${2}")
		snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
		objectfileName := strings.ToLower(snake)
		prevFile := filepath.Join(caller, v.Dir, fmt.Sprintf("%s.%s.%s", objectfileName, "generated", "go"))
		_, err := os.Stat(prevFile)
		if err == nil {
			err = os.Remove(prevFile)
			if err != nil {
				return fmt.Errorf("couldn't remove previous %s.generated and it's required to pass the validation: %w", v.Name, err)
			}
		}
	}
	err = config.pkgData(caller)
	if err != nil {
		return err
	}
	err = config.valid(caller)
	if err != nil {
		return err
	}
	g.config = config
	return nil
}

func (o *Config) valid(caller string) error {
	if o.RootDir == "" {
		return fmt.Errorf("the dir of the root objects pkg is required")
	}
	if o.RootPkg == "" {
		return fmt.Errorf("the root pkg is required")
	}
	_, err := os.Stat(filepath.Join(caller, o.RootDir))
	if err != nil {
		return fmt.Errorf("the root pkg dir must exist %w", err)
	}
	for _, v := range o.Objects {
		pkg, ok := o.PkgData[v.Pkg]
		if !ok {
			return fmt.Errorf("the package %s is not present in PkgData", v.Pkg)
		}
		err = v.valid(pkg)
		if err != nil {
			return err
		}
	}
	if o.Db != nil {
		pkg, ok := o.PkgData[o.Db.Pkg]
		if !ok {
			return fmt.Errorf("the package %s is not present in PkgData", o.Db.Pkg)
		}
		err := o.Db.valid(pkg)
		if err != nil {
			return err
		}
	}
	return nil
}

// Checks if the dir exists.
// Parses the dir into an ast.
// Inspect the ast and determine if the pkg and builder method exists.
// and the builder method has a the correct signature
// the expected signature is func() (*pgpool.Pool, error)
// where pgpool is imported from jackc/pgx/v5.
func (o DbConfig) valid(pkgData *PkgData) error {
	if o.Pkg == "" {
		return fmt.Errorf("db config obj is present db pkg name is required")
	}
	if o.Dir == "" {
		return fmt.Errorf("db config obj is present db pkg dir is required")
	}
	if o.Builder == "" {
		return fmt.Errorf("db obj is present builder method name is required")
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
	for _, file := range pkgData.files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if n.Name.Name == o.Builder {
					isCorrect = checkSig(pkgData.info.Defs[n.Name].Type().(*types.Signature))
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

// Check for nil fields in the object type, all fields with the exception of ValidatedFields are required.
// Parses the caller/objectType.Dir, look for the package with name equal to objectType.Pkg.
// Inside the package, find the objectType.Name,it must be a concrete struct type with all
// the specified unexported fields.
// Analyze the methods set of the struct, it must not contain a method for each
// field that takes no parameters, returns only the field data type and is
// named after the field with the first letter capital cased(getter).
// Searchs for a function with name equal objectType.Builder , it has to
// accept as many parameters as specified fields typed in the same order as the
// objectType.Fields slice (configuration file) and to return a single value
// with its type equal to (objectType.Pkg).(objectType.Name).
// If all this requirements are met, ObjectType is considered valid and the return is nil,
// otherwise it returns an error.
// If update or lazy loading are set to true the object must not contain a method
// for the field that takes one parameter of the same type the field have, return void
// and is named after the field with the first letter capital cased and the suffix Set (setter).
// If lazy loading is set to true, the object must have a field of type
// lazy_loading.lazyLoading and the builder must initialize this value LOADED.
func (o *ObjectType) valid(pkgData *PkgData) error {
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
	obj := pkgData.pkg.Scope().Lookup(o.Name)
	if obj == nil {
		return fmt.Errorf("could not find the object %s in %s", o.Name, pkgData.pkg.Path())
	}

	expectedFields := make(map[string]*ValidatedField, 0)
	for _, v := range o.Fields {
		expectedFields[v.Name] = &ValidatedField{update: v.Update}
	}

	if ctype, ok := obj.Type().Underlying().(*types.Struct); ok {
		for i := range ctype.NumFields() {
			v := ctype.Field(i)
			if efield, ok := expectedFields[v.Name()]; ok {
				name := v.Name()
				dataType := v.Type().String()
				efield.name = &name
				efield.dataType = &dataType
			}
		}
	}

	mset := types.NewMethodSet(obj.Type())
	checkReturn := func(tuple *types.Tuple, expected *ValidatedField) bool {
		if tuple.Len() != 1 {
			return false
		}
		rType := tuple.At(0).Type()
		return rType.String() == *expected.dataType
	}

	for i := range mset.Len() {
		meth := mset.At(i).Obj()
		methName := meth.Name()
		if efield, ok := expectedFields[strings.ToLower(methName)]; ok && meth.Exported() {
			if sig, ok := meth.Type().(*types.Signature); ok {
				if checkReturn(sig.Results(), efield) {
					efield.getterName = &methName
				}
			}
		}
	}

	expectedParams := slices.Collect(maps.Values(expectedFields))

	checkBuilderSig := func(sig *types.Signature) bool {
		params := sig.Params()
		if params.Len() != len(o.Fields) {
			return false
		}
		for i := range params.Len() {
			param := params.At(i)
			expectedParam := expectedParams[i]
			if param.Type().String() != *expectedParam.dataType {
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

	checkSetterSig := func(sig *types.Signature, efield *ValidatedField) bool {
		if sig.Results().Len() > 0 {
			return false
		}
		if sig.Params().Len() != 1 {
			return false
		}
		param := sig.Params().At(0)
		return param.Type().String() == *efield.dataType
	}

	isCorrect := false

	for _, file := range pkgData.files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch n := n.(type) {
			case *ast.FuncDecl:
				if n.Name.Name == o.Builder {
					isCorrect = checkBuilderSig(pkgData.info.Defs[n.Name].Type().(*types.Signature))
				}
				if sig, ok := pkgData.info.Defs[n.Name].Type().(*types.Signature); ok &&
					n.Name.IsExported() && n.Recv != nil && len(n.Recv.List) == 1 {
					recv, ok := n.Recv.List[0].Type.(*ast.StarExpr)
					if ok {
						if forStruct, ok := recv.X.(*ast.Ident); ok && forStruct.Name == o.Name {
							fieldName := strings.ToLower(n.Name.Name[3:])
							if field, ok := expectedFields[fieldName]; ok &&
								checkSetterSig(sig, field) {
								field.setterName = &n.Name.Name
							}
						}
					}
				}
			}
			return true
		})
	}

	err = nil
	for k, v := range expectedFields {
		if v.getterName != nil {
			if err != nil {
				err = fmt.Errorf("%w\nthe field %s from type %s already have a getter method of type func () %s",
					err, k, o.Name, *v.dataType)
			} else {
				err = fmt.Errorf("\nthe field %s from type %s already have a getter method of type func () %s",
					k, o.Name, *v.dataType)
			}
		}
		if v.name == nil {
			if err != nil {
				err = fmt.Errorf("%w\nthe field %s is not present in type %s",
					err, k, o.Name)
			} else {
				err = fmt.Errorf("\nthe field %s is not present in type %s",
					k, o.Name)
			}
		}
		if v.dataType == nil {
			if err != nil {
				err = fmt.Errorf("%w\ncould not get the data type for the field %s of type %s",
					err, k, o.Name)
			} else {
				err = fmt.Errorf("\ncould not get the data type for the field %s of type %s",
					k, o.Name)
			}
		}
		if v.update && v.setterName != nil {
			if err != nil {
				err = fmt.Errorf("%w\nthe field %s has an update flag and already have a public set method for type %s",
					err, k, o.Name)
			} else {
				err = fmt.Errorf("\nthe field %s has an update flag, and already have a public set method for type %s",
					k, o.Name)
			}
		}
	}

	if err != nil {
		return err
	}

	if !isCorrect {
		return fmt.Errorf("the builder method %s for type %s found in package %s has an incorrect signature",
			o.Builder, o.Name, o.Pkg)
	}

	o.ValidatedFields = slices.Collect(maps.Values(expectedFields))
	return nil
}
