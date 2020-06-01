package main

import (
	"bufio"
	"bytes"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

func main() {
	s, err := NewStore(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	store, err := walk(s.Dir)
	if err != nil {
		log.Fatal(err)
	}
	s.SetStore(store)
	http.HandleFunc("/", render(s))
	http.HandleFunc("/edit/", edit(s))
	http.Handle("/static/", http.StripPrefix("/static/",
		http.FileServer(http.Dir("static"))))
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

func render(s *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pageTmpl := template.Must(template.ParseFiles("templates/layout.html"))
		rd, ok := s.Get(r.URL.Path)
		if !ok {
			name := filepath.Join(s.Dir, r.URL.Path)
			http.ServeFile(w, r, name)
			return
		}
		err := pageTmpl.Execute(w, map[string]interface{}{
			"Name": r.URL.Path,
			"Page": template.HTML(rd),
		})
		if err != nil {
			log.Println("couldn't generate template ", err)
		}
	}
}

func walk(dir string) (map[string][]byte, error) {
	store := make(map[string][]byte)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// prevent panic by handling failure accessing a path
			return err
		}
		if toParse(info.Name()) {
			f, err := os.Open(path)
			if err != nil {
				log.Fatal("open: ", err)
			}
			defer f.Close()
			log.Println("parsing ", path)
			r, err := parse(dir, f)
			if err != nil {
				log.Fatal("parse: ", err)
			}
			key := path[len(dir):]
			//TODO check err
			by, err := ioutil.ReadAll(r)
			if err != nil {
				return err
			}
			store[key] = by
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return store, nil
}

func toParse(name string) bool {
	ext := filepath.Ext(name)
	return ext == ".txt" || ext == ".md"
}

func parse(baseDir string, in io.Reader) (io.Reader, error) {
	r := bufio.NewReader(in)
	out := &bytes.Buffer{}
	lineNo := 1
	for {
		line, err := r.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, fmt.Errorf("ReadLine: %w", err)
		}
		content := ""
		if ismacro(line) {
			output, err := macro(removeMacroSyntax(line), baseDir)
			if err != nil {
				return nil, err
			}
			for _, l := range strings.Split(output, "\n") {
				content += fmt.Sprintf(`<div>%s</div>`, toHTML(l))
			}
		} else {
			content = toHTML(line)
		}
		out.Write([]byte(fmt.Sprintf(`<div class="line" id="%d">%s</div>`, lineNo, content)))
		lineNo++
		if err == io.EOF {
			break
		}
	}
	return out, nil
}

func removeMacroSyntax(line string) string {
	return macroReg.ReplaceAllString(line, "")
}

var (
	macroReg = regexp.MustCompile(`^[\t ]*\#\|`)
	ismacro  = macroReg.MatchString
)

func macro(command string, dir string) (string, error) {
	out := &bytes.Buffer{}
	cmd := exec.Command("bash", "-c", command)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("command run: %w", err)
	}
	return out.String(), nil
}

var (
	taskReg = regexp.MustCompile(`^[\t ]*[\-\#] \[([ a-zA-Z]*)\] `)
	istask  = taskReg.MatchString
	linkReg = regexp.MustCompile(`\[(.*)\]\((.*)\)`)
)

func toHTML(line string) string {
	switch {
	case strings.HasPrefix(line, "# "):
		line = fmt.Sprintf(`<span class="heading">%s</span>`, strings.TrimLeft(line, "# "))
	case istask(line):
		line = taskReg.ReplaceAllString(line, `<span class="keyword">$1</span>`)
	}
	if linkReg.MatchString(line) {
		line = linkReg.ReplaceAllString(line, `<a href="$2">$1</a>`)
	}
	return line
}

func edit(s *Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			if err := update(s.Dir, r); err != nil {
				w.Write([]byte("something went wrong"))
				log.Println(err)
				return
			}
			go func() {
				parsed, err := walk(s.Dir)
				if err != nil {
					log.Println("=== WARNING ===\n%s", err)
					return
				}
				s.SetStore(parsed)
			}()
		case http.MethodGet:
			if err := renderEdit(s.Dir, w, r); err != nil {
				w.Write([]byte("something went wrong"))
				log.Println(err)
				return
			}
		default:
			w.Write([]byte(http.StatusText(http.StatusMethodNotAllowed)))
		}
	}
}

func update(dir string, r *http.Request) error {
	fileName := filepath.Join(dir, r.URL.Path[(len("/edit")):])
	f, err := os.OpenFile(fileName, os.O_WRONLY|os.O_TRUNC, os.ModePerm)
	if err != nil {
		return fmt.Errorf("open: %s", err)
	}
	defer f.Close()
	_, err = io.Copy(f, r.Body)
	return err
}

func renderEdit(dir string, w http.ResponseWriter, r *http.Request) error {
	//TODO move to global when the template is better
	editTmpl := template.Must(template.ParseFiles("templates/edit.html"))
	path := r.URL.Path[(len("/edit")):]
	fileName := filepath.Join(dir, path)
	f, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("open: %s", err)
	}
	defer f.Close()
	content, err := ioutil.ReadAll(f)
	if err != nil {
		return fmt.Errorf("readall: %s", err)
	}
	err = editTmpl.Execute(w, map[string]interface{}{
		"Name":    path,
		"Content": string(content),
	})
	if err != nil {
		return fmt.Errorf("template exec: %s", err)
	}
	return nil
}
