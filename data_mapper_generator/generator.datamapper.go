package data_mapper_generator

import (
	"fmt"
	"path/filepath"
	"strings"
)

func (g *DataMapperGenerator) generateImports(o *ObjectType) {
	requiredImports := []string{
		"fmt",
		"github.com/jackc/pgx/v5",
		"github.com/jackc/pgx/v5/pgxpool",
		"clearly-not-a-secret-project/data_mapper",
		"clearly-not-a-secret-project/interfaces",
	}
	if o.Lazy {
		requiredImports = append(requiredImports, "reflect")
	}
	objPkgPath := fmt.Sprintf("%s/%s", filepath.Base(g.caller), o.Dir)
	allImports := make([]string, 0)
	allImports = append(allImports, objPkgPath)
	allImports = append(allImports, requiredImports...)
	g.wln("import (")
	for _, v := range allImports {
		g.wln(fmt.Sprintf("\"%s\"", v))
	}
	g.wln(")")
}

func (g *DataMapperGenerator) findStmt(o *ObjectType) string {
	columns := make([]string, 0, len(o.Fields))
	for i := range o.Fields {
		columns = append(columns, o.Fields[i].Column)
	}
	columnNames := strings.Join(columns, ", ")
	stmt := fmt.Sprintf(`SELECT %v FROM %s WHERE ID = $1;`, columnNames, o.Table)
	return stmt
}

func (g *DataMapperGenerator) insertStmt(o *ObjectType) string {
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

func (g *DataMapperGenerator) updateStmt(o *ObjectType) string {
	columns := make([]string, 0, len(o.Fields))
	for i := range o.Fields {
		if o.Fields[i].Update {
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

func (g *DataMapperGenerator) removeStmt(o *ObjectType) string {
	stmt := fmt.Sprintf(`DELETE FROM %s WHERE ID = $1;`, o.Table)
	return stmt
}

func (g *DataMapperGenerator) generateDataMapperStructType(o *ObjectType) {
	index := -1
	for i := range o.ValidatedFields {
		if *o.ValidatedFields[i].name == "id" {
			index = i
		}
	}
	if index < 0 {
		panic(fmt.Errorf("could not find id field in the validated fields"))
	}
	idField := o.ValidatedFields[index]
	g.wln(fmt.Sprintf("type %sDataMapper struct {", o.Name))
	g.wln(fmt.Sprintf("%s.PostgreSQLDataMapper[%s.DomainObject[%s],%s]",
		dataMapperPkg, interfacesPkg, *idField.dataType, *idField.dataType,
	))
	g.wln("}")
}

func (g *DataMapperGenerator) generateDataMapperLazy(o *ObjectType) {
	index := -1
	for i := range o.ValidatedFields {
		if *o.ValidatedFields[i].name == "id" {
			index = i
		}
	}
	if index < 0 {
		panic(fmt.Errorf("could not find id field in the validated fields"))
	}
	idField := o.ValidatedFields[index]
	g.wln("LazyLoading: true,")
	g.wln(fmt.Sprintf("CreateGhost: %s.Create%sGhost,", o.Pkg, o.Name))
	g.wln(fmt.Sprintf(`
	DoLoadLine: func(resultSet pgx.Rows, obj %s.DomainObject[%s]) error {
	`, interfacesPkg, *idField.dataType))
	g.wln(fmt.Sprintf(`
	subject,ok := obj.(*%s.%s)
	`, o.Pkg, o.Name))
	g.wln(`
	if !ok {
		return fmt.Errorf("wrong type assertion")
	}
	`)
	g.wln("var (")
	for _, v := range o.ValidatedFields {
		g.wln(fmt.Sprintf("%s %s", *v.name, *v.dataType))
	}
	g.wln(")")
	g.wln("err := resultSet.Scan(")
	for _, v := range o.ValidatedFields {
		g.wln(fmt.Sprintf("&%s,", *v.name))
	}
	g.wln(")")
	g.wln(fmt.Sprintf(`
	if err != nil {
	return fmt.Errorf("error at doLoadLine %%w\n",err)
	}
	`))
	for _, v := range o.ValidatedFields {
		if *v.name != "id" {
			n := matchFirstCh.ReplaceAllStringFunc(*v.name, strings.ToUpper)
			g.wln(fmt.Sprintf("subject.Set%s(%s)", n, *v.name))
		}
	}
	g.wln("return nil")
	g.wln("},")
}

func (g *DataMapperGenerator) generateDataMapperCBuilder(o *ObjectType) {
	index := -1
	for i := range o.ValidatedFields {
		if *o.ValidatedFields[i].name == "id" {
			index = i
		}
	}
	if index < 0 {
		panic(fmt.Errorf("could not find id field in the validated fields"))
	}
	idField := o.ValidatedFields[index]
	g.wln(fmt.Sprintf(
		`func New%sDataMapper(pool *pgxpool.Pool,loadedMap map[%s]%s.DomainObject[%s],) *%sDataMapper {`,
		o.Name, *idField.dataType, interfacesPkg, *idField.dataType, o.Name,
	))
	g.wln(fmt.Sprintf(
		"return &%sDataMapper{",
		o.Name,
	))
	g.wln(fmt.Sprintf(
		"PostgreSQLDataMapper: %s.PostgreSQLDataMapper[%s.DomainObject[%s],%s]{",
		dataMapperPkg, interfacesPkg, *idField.dataType, *idField.dataType,
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
	g.wln(fmt.Sprintf("DomainType: reflect.TypeOf(&%s.%s{}),", o.Pkg, o.Name))
	if o.Lazy {
		g.generateDataMapperLazy(o)
	}
	g.wln("},")
	g.wln("}")
	g.wln("}")
}

func (g *DataMapperGenerator) generateDoUpdateFn(o *ObjectType) {
	index := -1
	for i := range o.ValidatedFields {
		if *o.ValidatedFields[i].name == "id" {
			index = i
		}
	}
	if index < 0 {
		panic(fmt.Errorf("could not find id field in the validated fields"))
	}
	idField := o.ValidatedFields[index]
	g.wln(fmt.Sprintf(
		"DoUpdate: func(obj %s.DomainObject[%s], stmt *%s.PreparedStatement) error {",
		interfacesPkg, *idField.dataType, dataMapperPkg,
	))
	g.wln(fmt.Sprintf(
		"subject, ok := obj.(*%s.%s)", o.Pkg, o.Name,
	))
	g.wln("if !ok { return fmt.Errorf(\"wrong type assertion\")}")
	g.wln("stmt.Append(subject.Id())")
	for _, v := range o.ValidatedFields {
		if v.update {
			n := matchFirstCh.ReplaceAllStringFunc(*v.name, strings.ToUpper)
			g.wln(fmt.Sprintf("stmt.Append(subject.%s())", n))
		}
	}
	g.wln("return nil },")
}

func (g *DataMapperGenerator) generateDoInsertFn(o *ObjectType) {
	index := -1
	for i := range o.ValidatedFields {
		if *o.ValidatedFields[i].name == "id" {
			index = i
		}
	}
	if index < 0 {
		panic(fmt.Errorf("could not find id field in the validated fields"))
	}
	idField := o.ValidatedFields[index]
	g.wln(fmt.Sprintf(
		"DoInsert: func(obj %s.DomainObject[%s], stmt *%s.PreparedStatement) error {",
		interfacesPkg, *idField.dataType, dataMapperPkg,
	))
	g.wln(fmt.Sprintf(
		"subject, ok := obj.(*%s.%s)",
		o.Pkg, o.Name,
	))
	g.wln("if !ok { return fmt.Errorf(\"wrong type assertion \") }")
	for _, v := range o.ValidatedFields {
		n := matchFirstCh.ReplaceAllStringFunc(*v.name, strings.ToUpper)
		g.wln(fmt.Sprintf("stmt.Append(subject.%s())", n))
	}
	g.wln("return nil },")
}

func (g *DataMapperGenerator) generateDoLoadFn(o *ObjectType) {
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
		"DoLoad: func (resultSet pgx.Rows) (%s.DomainObject[%s],error){",
		interfacesPkg, *idField.dataType,
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
	g.wln("if err != nil {return nil, err}")
	g.wln(fmt.Sprintf("return %s.%s(", o.Pkg, o.Builder))
	for k := range variables {
		g.wln(fmt.Sprintf("%s,", k))
	}
	g.wln("), nil")
	g.wln("},")
}

func (g *DataMapperGenerator) generateDataMapper(o *ObjectType) error {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
		}
	}()
	g.buff.Reset()
	newPkgPath := g.generateNewPkg(generatedPkgName, generatedPkgName)
	g.generateImports(o)
	g.generateDataMapperStructType(o)
	g.generateDataMapperCBuilder(o)
	err := g.writeFile(newPkgPath, o.Name, "data_mapper", "")
	if err != nil {
		return err
	}
	return nil
}
