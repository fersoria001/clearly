package data_mapper_generator

import (
	"fmt"
)

func (g *DataMapperGenerator) generateDataSourceImports() {
	requiredImports := []string{
		fmt.Sprintf("clearly-not-a-secret-project/%s", generatedRegistryPkg),
		"clearly-not-a-secret-project/interfaces",
		"fmt",
	}
	g.wln("import (")
	for _, v := range requiredImports {
		g.wln(fmt.Sprintf("\"%s\"", v))
	}
	g.wln(")")
}

func (g *DataMapperGenerator) generateDataSource() error {
	g.buff.Reset()
	path := g.generateNewPkg(g.config.RootDir, g.config.RootPkg)
	g.generateDataSourceImports()
	g.wln(fmt.Sprintf(`
	func Load[K comparable](obj interfaces.DomainObject[K]) error {
		instance,err := %s.Instance[K]()
		if err != nil {
		return fmt.Errorf("error in concrete datasource %%w", err)
		}
		err = instance.Load(obj)
		if err != nil {
		return fmt.Errorf("error in concrete datasource %%w",err)
		}
		return nil
	}
	`, generatedRegistryPkg))

	err := g.writeFile(path, "generated", "", "datasource")
	if err != nil {
		return err
	}
	return nil
}
