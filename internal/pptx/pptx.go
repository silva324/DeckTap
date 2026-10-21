// Package pptx reads slide titles, speaker notes and the hidden flag directly from a
// .pptx/.ppsx/.pptm file (Office Open XML), with no Office installation required.
package pptx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"
)

const drawingNS = "http://schemas.openxmlformats.org/drawingml/2006/main"

// Slide is one slide in presentation order.
type Slide struct {
	Index  int    `json:"n"`
	Title  string `json:"title"`
	Notes  string `json:"-"`
	Hidden bool   `json:"hidden"`
}

const maxPartSize = 32 << 20 // defensive cap per XML part

// Read parses the file at path.
func Read(filePath string) ([]Slide, error) {
	zr, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	return read(&zr.Reader)
}

// ReadBytes parses an in-memory file.
func ReadBytes(b []byte) ([]Slide, error) {
	zr, err := zip.NewReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		return nil, err
	}
	return read(zr)
}

func read(zr *zip.Reader) ([]Slide, error) {
	files := map[string]*zip.File{}
	for _, f := range zr.File {
		files[f.Name] = f
	}
	load := func(name string) ([]byte, error) {
		f, ok := files[name]
		if !ok {
			return nil, fmt.Errorf("pptx: missing part %s", name)
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		defer rc.Close()
		return io.ReadAll(io.LimitReader(rc, maxPartSize))
	}

	pres, err := load("ppt/presentation.xml")
	if err != nil {
		return nil, err
	}
	presRels, err := load("ppt/_rels/presentation.xml.rels")
	if err != nil {
		return nil, err
	}
	rels := parseRels(presRels, "ppt")

	var order []string
	dec := xml.NewDecoder(bytes.NewReader(pres))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if se, ok := tok.(xml.StartElement); ok && se.Name.Local == "sldId" {
			for _, a := range se.Attr {
				if a.Name.Local == "id" && strings.Contains(a.Name.Space, "relationships") {
					if target, ok := rels[a.Value]; ok {
						order = append(order, target.path)
					}
				}
			}
		}
	}

	slides := make([]Slide, 0, len(order))
	for i, slidePath := range order {
		s := Slide{Index: i + 1}
		data, err := load(slidePath)
		if err != nil {
			slides = append(slides, s)
			continue
		}
		shapes, hidden := parseShapes(data)
		s.Hidden = hidden
		for _, sh := range shapes {
			if sh.ph == "title" || sh.ph == "ctrTitle" {
				s.Title = sh.text
				break
			}
		}
		relsPath := path.Join(path.Dir(slidePath), "_rels", path.Base(slidePath)+".rels")
		if rb, err := load(relsPath); err == nil {
			for _, r := range parseRels(rb, path.Dir(slidePath)) {
				if strings.HasSuffix(r.typ, "/notesSlide") {
					if nb, err := load(r.path); err == nil {
						notesShapes, _ := parseShapes(nb)
						for _, sh := range notesShapes {
							if sh.ph == "body" {
								s.Notes = sh.text
								break
							}
						}
						if s.Notes == "" {
							for _, sh := range notesShapes {
								if sh.ph != "sldImg" && sh.ph != "sldNum" && sh.ph != "hdr" && sh.ph != "ftr" && sh.ph != "dt" && sh.text != "" {
									s.Notes = sh.text
									break
								}
							}
						}
					}
				}
			}
		}
		slides = append(slides, s)
	}
	return slides, nil
}

type rel struct{ path, typ string }

func parseRels(b []byte, baseDir string) map[string]rel {
	out := map[string]rel{}
	dec := xml.NewDecoder(bytes.NewReader(b))
	for {
		tok, err := dec.Token()
		if err != nil {
			return out
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != "Relationship" {
			continue
		}
		var id, target, typ, mode string
		for _, a := range se.Attr {
			switch a.Name.Local {
			case "Id":
				id = a.Value
			case "Target":
				target = a.Value
			case "Type":
				typ = a.Value
			case "TargetMode":
				mode = a.Value
			}
		}
		if mode == "External" {
			continue
		}
		p := target
		if strings.HasPrefix(p, "/") {
			p = strings.TrimPrefix(p, "/")
		} else {
			p = path.Clean(path.Join(baseDir, p))
		}
		out[id] = rel{path: p, typ: typ}
	}
}

type shape struct {
	ph   string
	text string
}

// parseShapes returns shapes with their placeholder type and text, plus the hidden flag
// of the slide root (show="0").
func parseShapes(b []byte) ([]shape, bool) {
	var shapes []shape
	hidden := false
	dec := xml.NewDecoder(bytes.NewReader(b))
	var cur *shape
	var sb strings.Builder
	paragraphs := 0
	inText := false
	root := true
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if root {
				root = false
				for _, a := range t.Attr {
					if a.Name.Local == "show" && (a.Value == "0" || a.Value == "false") {
						hidden = true
					}
				}
			}
			switch {
			case t.Name.Local == "sp" && t.Name.Space != drawingNS:
				cur = &shape{}
				sb.Reset()
				paragraphs = 0
			case cur == nil:
			case t.Name.Local == "ph":
				cur.ph = "body" // a placeholder without type is a body/content placeholder
				for _, a := range t.Attr {
					if a.Name.Local == "type" {
						cur.ph = a.Value
					}
				}
			case t.Name.Space == drawingNS && t.Name.Local == "p":
				if paragraphs > 0 {
					sb.WriteByte('\n')
				}
				paragraphs++
			case t.Name.Space == drawingNS && t.Name.Local == "br":
				sb.WriteByte('\n')
			case t.Name.Space == drawingNS && t.Name.Local == "t":
				inText = true
			}
		case xml.CharData:
			if inText && cur != nil {
				sb.Write(t)
			}
		case xml.EndElement:
			if t.Name.Space == drawingNS && t.Name.Local == "t" {
				inText = false
			}
			if t.Name.Local == "sp" && t.Name.Space != drawingNS && cur != nil {
				cur.text = strings.TrimSpace(sb.String())
				shapes = append(shapes, *cur)
				cur = nil
			}
		}
	}
	return shapes, hidden
}
