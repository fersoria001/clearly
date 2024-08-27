package data_mapper_generator

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"log"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	"golang.org/x/tools/imports"
)

const configFileName = "config.json"
const dataMapperPkg = "data_mapper"
const interfacesPkg = "interfaces"
const generatedPkgName = "generated"
const generatedTestPkgName = "generated_tests"
const generatedRegistryPkg = "generated_registry"

var matchFirstCh = regexp.MustCompile("^[a-zA-Z]")
var matchFirstCap = regexp.MustCompile("(.)([A-Z][a-z]+)")
var matchAllCap = regexp.MustCompile("([a-z0-9])([A-Z])")

type DataMapperGenerator struct {
	caller string
	config *Config
	buff   *bytes.Buffer
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
	g := new(DataMapperGenerator)
	log.Println("reading and validating the configuration it can take a while...")
	err := g.readConfig(caller)
	if err != nil {
		return nil, err
	}
	log.Println("done")
	g.caller = caller
	g.buff = bytes.NewBuffer(make([]byte, 0))
	return g, nil
}

func (g *DataMapperGenerator) wln(w string) {
	_, err := g.buff.WriteString(fmt.Sprintf("%s\n", w))
	if err != nil {
		panic(err)
	}
}

func (g *DataMapperGenerator) GenerateAll() error {
	for i := range g.config.Objects {
		log.Printf("generating object methods for object: %s...\n", g.config.Objects[i].Name)
		err := g.generateObjectMethods(g.config.Objects[i])
		if err != nil {
			return err
		}
		log.Println("done")
		log.Printf("generating data mapper for object: %s...\n", g.config.Objects[i].Name)
		err = g.generateDataMapper(g.config.Objects[i])
		if err != nil {
			return err
		}
		log.Println("done")
	}
	if g.config.Db == nil {
		return fmt.Errorf("the db config is not defined")
	}
	log.Println("generating the data mapper's registry...")
	err := g.generateDataMapperRegistry()
	if err != nil {
		return err
	}
	log.Println("done")
	log.Println("generating a domain data source...")
	err = g.generateDataSource()
	if err != nil {
		return err
	}
	log.Println("done")
	err = g.generateTestErrorsFile()
	if err != nil {
		return err
	}
	for i := range g.config.Objects {
		log.Printf("generating the %s data mapper test...", g.config.Objects[i].Name)
		err := g.generateTest(g.config.Objects[i])
		if err != nil {
			return err
		}
		log.Println("done")
	}

	p, err := filepath.Abs(g.caller)
	if err != nil {
		return err
	}
	log.Println("running all data mappers tests")
	out, err := exec.Command("go", "test", filepath.Join(p, generatedTestPkgName)).CombinedOutput()
	if err != nil {
		return err
	}
	log.Print(string(out))
	log.Println("code generated.")
	return nil
}

// Creates the folder for the package in caller/pkg
// if already exists it skips this step,
// if err it panics else writes package name to the generator buffer
// returns the absolute to the current project path to the pkg.
func (g *DataMapperGenerator) generateNewPkg(dir, pkgName string) string {
	generatedDir := filepath.Join(g.caller, dir)
	err := os.MkdirAll(generatedDir, os.ModePerm)
	if err != nil {
		panic(err)
	}
	g.wln(fmt.Sprintf("package %s", pkgName))
	return generatedDir
}

// For each object parses the dir, look for the package if found type check it
// and return the pkg, the pkg files, type.Info with defs and types and nil
// or an error if the target package is not found
func (c *Config) pkgData(caller string) error {
	fromConfig := make(map[string]string, 0)
	for _, v := range c.Objects {
		fromConfig[v.Pkg] = v.Dir
	}
	fromConfig[c.Db.Pkg] = c.Db.Dir
	for k, v := range fromConfig {
		dir := filepath.Join(caller, v)
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
			return fmt.Errorf("\n dbConfig dir:\n %s \n is not a valid go source file: %w", dir, err)
		}
		conf := types.Config{
			Importer: importer.ForCompiler(fset, "source", nil),
		}
		info := &types.Info{
			Defs:  make(map[*ast.Ident]types.Object),
			Types: make(map[ast.Expr]types.TypeAndValue),
		}
		if pkg, ok := pkgs[k]; ok {
			files := slices.Collect(maps.Values(pkg.Files))
			typesPkg, err := conf.Check(dir, fset, files, info)
			if err != nil {
				return fmt.Errorf("\ntype checking err in %s:\n\t %w", dir, err)
			}
			c.PkgData[k] = &PkgData{
				pkg:   typesPkg,
				files: files,
				info:  info,
			}
		}
	}
	return nil
}

func (g *DataMapperGenerator) writeFile(pkgName, objectName, suffix, dotSufix string) error {
	snake := matchFirstCap.ReplaceAllString(objectName, "${1}_${2}")
	snake = matchAllCap.ReplaceAllString(snake, "${1}_${2}")
	newFileName := strings.ToLower(snake)
	if suffix != "" {
		newFileName += fmt.Sprintf("_%s", suffix)
	}
	if dotSufix != "" {
		newFileName += fmt.Sprintf(".%s", dotSufix)
	}
	newFileName += ".go"
	newPath := filepath.Join(pkgName, newFileName)
	formatted, err := imports.Process(newPath, g.buff.Bytes(), &imports.Options{Comments: true})
	if err != nil {
		return fmt.Errorf("error at formatting imports %w", err)
	}
	err = os.WriteFile(newPath, formatted, os.ModePerm)
	if err != nil {
		return err
	}
	return nil
}
