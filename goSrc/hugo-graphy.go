package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/learninfun/hugo-graphy/parser/metadecoders"
	"github.com/learninfun/hugo-graphy/tools/tools_check"
	"github.com/learninfun/hugo-graphy/tools/tools_convert"
	"github.com/learninfun/hugo-graphy/tools/tools_other"
)

var (
	siteFolder       string
	sourseFolder     string
	langSourceFolder string
	langNow          string //currently handle lang
	resultFolder     string
	sourseFolderLen  int
	pathMK           string
	InLinkRegex      *regexp.Regexp
	cardMap          map[string]Card
)

type Card struct {
	Name   string   `json:"name"`
	Url    string   `json:"url"`
	Md     string   `json:"md"`
	Link   []string `json:"to"`
	Linked []string `json:"fm"`
}

func main() {
	initGlobalVar()
	confMap := getConfMap()

	err := os.RemoveAll(resultFolder) //clear ori result
	check(err)

	graphyJsFolder := siteFolder + pathMK + "static" + pathMK + "hugo-graphy" + pathMK + "js"
	if !tools_check.FileExists(graphyJsFolder) {
		err := os.MkdirAll(graphyJsFolder, 0755)
		check(err)
	}

	makeFolderIfNotExist(resultFolder)

	languages, languagesFound := confMap["Languages"]
	if !languagesFound { //without language, just walk through souseFolder
		cardMap = make(map[string]Card)
		filepath.Walk(sourseFolder, walkAllFile)

		cardMapStr := tools_convert.JsonToString(confMap)
		tools_convert.StringToFile(graphyJsFolder+pathMK+"hugo-graphy-data.js", "hugoGraphyData="+cardMapStr)
	} else {
		languagesMap := languages.(map[string]any)
		for langName, langConf := range languagesMap { //loop all lang
			langNow = langName
			langConfMap := langConf.(map[string]any)
			fmt.Println("langName:", langName, "=>", "langConf:", langConfMap)
			langContentDir, langContentDirFound := langConfMap["contentDir"]
			if !langContentDirFound {
				fmt.Println(langContentDir)
				panic("Not support language without contentDir")
			}

			langContentDirStr := langContentDir.(string)
			langSourceFolder = siteFolder + pathMK + "contentOri" + pathMK + langContentDirStr[8:]

			cardMap = make(map[string]Card)
			filepath.Walk(langSourceFolder, walkAllFile)

			cardMapStrB := tools_convert.JsonToStringBeauty(cardMap)
			fmt.Println(cardMapStrB)
			cardMapStr := tools_convert.JsonToString(cardMap)
			tools_convert.StringToFile(graphyJsFolder+pathMK+"hugo-graphy-data_"+langName+".js", "hugoGraphyData="+cardMapStr)
		}
	}
}

func initGlobalVar() {
	var err error

	currentFolder, err := os.Getwd()
	check(err)

	if tools_check.FileExists(currentFolder + pathMK + "contentOri") {
		siteFolder = currentFolder
	} else {
		siteFolder = filepath.Dir(currentFolder) //如果執行目錄是在hugo-graphy那層，要先回到前一層
	}

	pathMK = string(os.PathSeparator)

	sourseFolder = siteFolder + pathMK + "contentOri"
	sourseFolderLen = len(sourseFolder)
	resultFolder = siteFolder + pathMK + "content"

	regexInLink := `\[\[[^\/\\\:\*\?\"\<\>\|\]]{1,254}\]\]`
	InLinkRegex, err = regexp.Compile(regexInLink)
	check(err)
}

func walkAllFile(pathSourse string, fileInfo os.FileInfo, err error) error {
	check(err)

	var pathRelative = pathSourse[sourseFolderLen:]
	var pathResult = resultFolder + pathRelative
	if fileInfo.IsDir() {
		//fmt.Printf("Folder Name: %s\n", pathRelative)
		makeFolderIfNotExist(pathResult)

		if langSourceFolder != pathSourse &&
			!tools_check.FileExists(pathSourse+pathMK+"_index.md") &&
			!tools_check.FileExists(pathSourse+pathMK+"_index.html") {
			fmt.Println("Create _index.md:" + pathSourse + pathMK + "_index.md")
			create_indexMd(fileInfo.Name(), pathResult+pathMK+"_index.md")
		}
	} else {
		//fmt.Printf("file Name: %s\n", pathRelative)
		handledMarkdown := handleMarkdown(fileInfo.Name(), pathSourse, pathResult)
		if !handledMarkdown {
			tools_other.FileCopy(pathSourse, pathResult)
		}
	}

	return nil
}

func handleMarkdown(fileName, pathSourse, pathResult string) bool {
	if strings.ToLower(filepath.Ext(pathSourse)) != ".md" { //only handle markdown
		return false
	}

	if fileName == "_index.md" {
		return false
	}

	oriContent := tools_convert.FileTostring(pathSourse)
	newContent := oriContent

	//get card for this page
	fileNameNoExt := tools_convert.FileNameNoExt(fileName)
	fmt.Println(fileNameNoExt)
	cardThisPage, fmCardFound := cardMap[fileNameNoExt]
	if !fmCardFound {
		cardThisPage = Card{Name: fileNameNoExt}
	}
	cardThisPage.Url = getPageUrl(pathSourse)
	cardThisPage.Md = tools_convert.FileTostring(pathSourse)

	//get all internal link
	regexMachesArr := InLinkRegex.FindAllStringSubmatchIndex(oriContent, -1)
	offsetIdx := 0
	for i := 0; i < len(regexMachesArr); i++ {
		linkStr := oriContent[regexMachesArr[i][0]+2 : regexMachesArr[i][1]-2]
		anchorStr := fmt.Sprintf("<a id='inlink%d' class='inlink'>%s</a>", i, linkStr)
		newContent = newContent[:regexMachesArr[i][0]+offsetIdx] + anchorStr + newContent[regexMachesArr[i][1]+offsetIdx:]
		offsetIdx += len(anchorStr) - (regexMachesArr[i][1] - regexMachesArr[i][0])

		//get card link from this page
		cardLink, cardLinkFound := cardMap[linkStr]
		if !cardLinkFound {
			cardLink = Card{Name: linkStr}
		}

		//create link in both direction
		cardLink.Linked = append(cardLink.Linked, fileNameNoExt)
		cardThisPage.Link = append(cardThisPage.Link, linkStr)

		cardMap[linkStr] = cardLink //save card to map
	}

	cardMap[fileNameNoExt] = cardThisPage //save card to map
	tools_convert.StringToFile(pathResult, newContent)

	return true
}

func getPageUrl(pathSourse string) string {
	result := langNow + pathSourse[len(langSourceFolder):]

	if result[:1] != pathMK {
		result = pathMK + result
	}

	result = result[:len(result)-len(filepath.Ext(result))]

	return filepath.ToSlash(result)
}

func create_indexMd(folderName, pathResult string) {
	content := fmt.Sprintf("+++\ntitle = \"%s\"\n+++", folderName)
	tools_convert.StringToFile(pathResult, content)
}

func check(e error) {
	if e != nil {
		panic(e)
	}
}

func makeFolderIfNotExist(folderPath string) {
	if !tools_check.FileExists(folderPath) {
		err := os.Mkdir(folderPath, 0755)
		check(err)
	}
}

func getConfMap() map[string]any {
	doc := tools_convert.FileTostring(siteFolder + pathMK + "config.toml")
	docBype := []byte(doc)
	opts := metadecoders.Default
	mapConf, err := opts.UnmarshalToMap(docBype, metadecoders.TOML)
	check(err)

	return mapConf
}