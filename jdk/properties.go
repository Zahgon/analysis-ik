package jdk

import (
	"encoding/xml"
	"errors"
	"io"
	"strings"
)

// Properties is the subset of java.util.Properties the analyzer uses: a string
// map loaded from the properties DTD's XML form.
type Properties map[string]string

// Get returns the value for key, and whether the key was present, mirroring
// getProperty returning null for an absent key.
func (p Properties) Get(key string) (string, bool) {
	v, ok := p[key]
	return v, ok
}

type propertiesXML struct {
	XMLName xml.Name        `xml:"properties"`
	Comment *string         `xml:"comment"`
	Entries []propertyEntry `xml:"entry"`
}

type propertyEntry struct {
	Key   *string `xml:"key,attr"`
	Value string  `xml:",chardata"`
}

// ErrInvalidPropertiesFormat reports a document that is not the properties DTD,
// which is what loadFromXML raises rather than silently reading it.
var ErrInvalidPropertiesFormat = errors.New("jdk: invalid properties format")

// LoadFromXML reads the
//
//	<properties><entry key="…">value</entry></properties>
//
// dialect that Properties.loadFromXML accepts. The document type declaration
// is skipped rather than fetched, and a UTF-8 byte order mark is tolerated —
// config/IKAnalyzer.cfg.xml ships with one and Java's parser accepts it.
func LoadFromXML(r io.Reader) (Properties, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	text := strings.TrimPrefix(string(data), "\uFEFF")
	var doc propertiesXML
	decoder := xml.NewDecoder(strings.NewReader(text))
	decoder.Strict = false
	if err := decoder.Decode(&doc); err != nil {
		// A document whose root is not <properties> fails here, which is the
		// same outcome loadFromXML produces for one the DTD rejects.
		return nil, err
	}
	props := make(Properties, len(doc.Entries))
	for _, entry := range doc.Entries {
		// The DTD makes key mandatory, and loadFromXML validates against it.
		if entry.Key == nil {
			return nil, ErrInvalidPropertiesFormat
		}
		props[*entry.Key] = entry.Value
	}
	return props, nil
}
