package main 

import (
    "os"
    "path"
    "strings"
    "io/ioutil"
    "log"
    "net/http"
    "html/template"
    "runtime/debug"
)

const (
    ListDir = 0x0001
    UPLOAD_DIR = "./uploads"
    TEMPLATE_DIR = "./views"
)   

var templates map[string]*template.Template

func init() {
    fileInfoArr,err := ioutil.ReadDir(TEMPLATE_DIR)
    if err != nil {
        panic(err)
        return
    }    

    templates = make(map[string]*template.Template)
    var templateName,templatePath string
    for _,fileInfo := range fileInfoArr {
        templateName = fileInfo.Name()
        if ext := path.Ext(templateName); ext != ".html" {
            continue
        }
        templatePath = TEMPLATE_DIR + "/" + templateName
        log.Println("Loading templates: ",templatePath)
        t := template.Must(template.ParseFiles(templatePath))
        tmpl := templateName[:strings.Index(templateName,".html")]
        templates[tmpl] = t

    }
}

func check(err error) {
    if err != nil {
        panic(err)
    }
}

func renderHtml(w http.ResponseWriter, tmpl string, locals map[string]interface{}) (err error) {
    err = templates[tmpl].Execute(w,locals)
    return
}

func uploadHandler(w http.ResponseWriter,r *http.Request) {
    if r.Method == "GET" {
        err := renderHtml(w,"board",nil)
        check(err)
        return
    }
}

func isExists(path string) bool {
    _,err := os.Stat(path)
    if err == nil {
        return true
    }
    return os.IsExist(err)
}

//import 
func safeHandler(fn http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter,r *http.Request) {
        defer func() {
            if e,ok := recover().(error); ok {
                http.Error(w,e.Error(),http.StatusInternalServerError)
                log.Println("WARN:panic in %v - %v",fn,e)
                log.Println(string(debug.Stack()))
            }
        }()

        fn(w,r)
    }
}

func staticDirHandler(mux *http.ServeMux, perfix string, staticDir string, flag int) {
    mux.HandleFunc(perfix,func(w http.ResponseWriter,r *http.Request){
        file := staticDir + r.URL.Path[len(perfix)-1:]
        log.Println(file)
        if (flag & ListDir) == 0 {
            if exists := isExists(file); !exists {
                http.NotFound(w,r)
                return
            }
        }
        http.ServeFile(w,r,file)
    })

    
}

func main() {
    mux := http.NewServeMux()
    staticDirHandler(mux,"/assets/","./public",0)
    mux.HandleFunc("/",safeHandler(uploadHandler))
    err := http.ListenAndServe(":9090",mux)
    if err != nil {
        log.Fatal("ListenAndServe: ",err.Error())
    }
}