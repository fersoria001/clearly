package data_mapper_generator

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (g *DataMapperGenerator) generateTestImports(o *ObjectType) {
	requiredImports := []string{
		"clearly-not-a-secret-project/interfaces",
		"testing",
		"context",
		"reflect",
	}
	registryPkg := fmt.Sprintf("%s/%s", filepath.Base(g.caller), generatedRegistryPkg)
	dbPkg := fmt.Sprintf("%s/%s", filepath.Base(g.caller), g.config.Db.Dir)
	objPkg := fmt.Sprintf("%s/%s", filepath.Base(g.caller), o.Dir)
	generatedPkg := fmt.Sprintf("%s/%s", filepath.Base(g.caller), generatedPkgName)
	allImports := make([]string, 0)
	allImports = append(allImports, objPkg)
	if dbPkg != objPkg {
		allImports = append(allImports, dbPkg)
	}
	allImports = append(allImports, registryPkg)
	allImports = append(allImports, generatedPkg)
	allImports = append(allImports, requiredImports...)
	g.wln("import (")
	for _, v := range allImports {
		g.wln(fmt.Sprintf("\"%s\"", v))
	}
	g.wln(")")
}

func (g *DataMapperGenerator) generateTestInsertFunc(o *ObjectType) {
	g.wln("t.Run(\"Insert\", func(t *testing.T) {")
	g.wln("for _, v := range testData {")
	g.wln(fmt.Sprintf(
		"aggregate := %s.%s(",
		o.Pkg, o.Builder,
	))
	for _, v := range o.ValidatedFields {
		n := matchFirstCh.ReplaceAllStringFunc(*v.name, strings.ToUpper)
		g.wln(fmt.Sprintf("v.%s,", n))
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

func (g *DataMapperGenerator) generateTestFindFunc(o *ObjectType) {
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
		n := matchFirstCh.ReplaceAllStringFunc(*v.name, strings.ToUpper)
		g.wln(fmt.Sprintf(
			`if aggregate.%s() != v.%s {
		t.Fatal(AssertionError{name: "%s", expected:v.%s, found:aggregate.%s()}.Error())
			}`,
			n, n, *v.name, n, n,
		))
	}
	g.wln("}})")
}

func (g *DataMapperGenerator) generateTestUpdateFunc(o *ObjectType) {
	g.wln("t.Run(\"Update\", func(t *testing.T) {")
	g.wln("for _,v := range testData {")
	g.wln("dbAggregate, err := dataMapper.Find(ctx, v.Id)")
	g.wln("if err != nil { t.Fatal(err) }")
	g.wln(fmt.Sprintf(
		"aggregate, ok := dbAggregate.(*%s.%s)",
		o.Pkg, o.Name,
	))
	g.wln("if !ok { t.Fatalf(\"wrong type assertion \")")
	g.wln("for _, v1 := range testUpdateData {")
	for _, v := range o.ValidatedFields {
		n := matchFirstCh.ReplaceAllStringFunc(*v.name, strings.ToUpper)
		if v.update {
			g.wln(fmt.Sprintf(
				"aggregate.Set%s(v1.%s)",
				n, n,
			))
		}
	}
	g.wln("}")
	g.wln("err = dataMapper.Update(ctx, aggregate)")
	g.wln("if err != nil { t.Fatal(err) }")
	g.wln("dbAggregate, err = dataMapper.Find(ctx, v.Id)")
	g.wln("if err != nil { t.Fatal(err) }")
	g.wln(fmt.Sprintf("aggregate, ok := dbAggregate.(*%s.%s)",
		o.Pkg, o.Name,
	))
	g.wln("if !ok { t.Fatalf(\"wrong type assertion \")")
	g.wln("for _, v1 := range testUpdateData {")
	for _, v := range o.ValidatedFields {
		if v.update {
			n := matchFirstCh.ReplaceAllStringFunc(*v.name, strings.ToUpper)
			g.wln(fmt.Sprintf(
				"if aggregate.%s() != v1.%s {",
				n, n,
			))
			g.wln(fmt.Sprintf(
				"t.Fatal(AssertionError{name: \"%s\", expected:v.%s, found:aggregate.%s()}.Error())",
				*v.name, n, n,
			))
			g.wln("}")
		}
	}
	g.wln("}")
	g.wln("}}}})")
}

func (g *DataMapperGenerator) generateTestRemoveFunc() {
	g.wln("t.Run(\"Remove\", func(t *testing.T) {")
	g.wln("for _,v := range testData {")
	g.wln("err := dataMapper.Remove(ctx, v.Id)")
	g.wln("if err != nil { t.Fatal(err) }")
	g.wln("}})")
}

func (g *DataMapperGenerator) generateTestFn(o *ObjectType) {
	index := -1
	variables := make(map[string]string, 0)
	for i := range o.ValidatedFields {
		if *o.ValidatedFields[i].name == "id" {
			index = i
		}
		variables[*o.ValidatedFields[i].name] = *o.ValidatedFields[i].dataType
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
	g.wln(fmt.Sprintf("pool, err := %s.%s()", g.config.Db.Pkg, g.config.Db.Builder))
	g.wln("if err != nil { t.Fatal(err) }")
	g.wln(fmt.Sprintf(
		"loadedMap := make(map[%s]%s.DomainObject[%s],0)",
		*idField.dataType, interfacesPkg, *idField.dataType,
	))
	g.wln(fmt.Sprintf(
		"newMapper := %s.New%sDataMapper(pool, loadedMap)",
		generatedPkgName, o.Name,
	))
	g.wln(fmt.Sprintf(
		"reg, err := %s.Instance[%s]()",
		generatedRegistryPkg, *idField.dataType,
	))
	g.wln("if err != nil { t.Fatal(err) }")
	g.wln("reg.Register(newMapper)")
	g.wln(fmt.Sprintf("dataMapper, err := reg.Mapper(reflect.TypeOf(&%s.%s{}))", o.Pkg, o.Name))
	g.wln("if err != nil { t.Fatal(err) }")
	g.generateTestInsertFunc(o)
	g.generateTestFindFunc(o)
	g.generateTestUpdateFunc(o)
	g.generateTestRemoveFunc()
	g.wln("}")
}

func (g *DataMapperGenerator) generateTestData(o *ObjectType) {
	g.wln("var testData = map[string] struct{")
	for _, v := range o.ValidatedFields {
		n := matchFirstCh.ReplaceAllStringFunc(*v.name, strings.ToUpper)
		g.wln(fmt.Sprintf("%s %s", n, *v.dataType))
	}
	g.wln("}{")
	g.wln("\"valid\": {")
	for _, v := range o.ValidatedFields {
		var randomValue any
		switch *v.dataType {
		case "string":
			randomValue = fmt.Sprintf("\"%s\"", randString(10))
		case "int":
			randomValue = randInt()
		}
		n := matchFirstCh.ReplaceAllStringFunc(*v.name, strings.ToUpper)
		g.wln(fmt.Sprintf(
			"%s: %v,",
			n, randomValue,
		))
	}
	g.wln("},}")

	g.wln("var testUpdateData = map[string] struct{")
	for _, v := range o.ValidatedFields {
		if v.update {
			n := matchFirstCh.ReplaceAllStringFunc(*v.name, strings.ToUpper)
			g.wln(fmt.Sprintf("%s %s", n, *v.dataType))
		}
	}
	g.wln("}{")
	g.wln("\"valid\": {")
	for _, v := range o.ValidatedFields {
		if v.update {
			var randomValue any
			switch *v.dataType {
			case "string":
				randomValue = fmt.Sprintf("\"%s\"", randString(10))
			case "int":
				randomValue = randInt()
			}
			n := matchFirstCh.ReplaceAllStringFunc(*v.name, strings.ToUpper)
			g.wln(fmt.Sprintf(
				"%s: %v,",
				n, randomValue,
			))
		}
	}
	g.wln("},}")

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
	newPkgPath := g.generateNewPkg(generatedTestPkgName, generatedTestPkgName)
	g.wln("import (")
	g.wln("\"fmt\"")
	g.wln(")")
	g.generateAssertionErrorType()
	err := g.writeFile(newPkgPath, "errors", "", "")
	if err != nil {
		return err
	}
	return nil
}

func (g *DataMapperGenerator) generateTest(o *ObjectType) error {
	g.buff.Reset()
	newPkgPath := g.generateNewPkg(generatedTestPkgName, generatedTestPkgName)
	g.generateTestImports(o)
	g.generateTestData(o)
	g.generateTestFn(o)
	err := g.writeFile(newPkgPath, o.Name, "data_mapper_test", "")
	if err != nil {
		return err
	}
	return nil
}
