package jdk

import (
	"errors"
	"strings"
	"testing"
)

const bomPrefixedConfig = "\uFEFF" + `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE properties SYSTEM "http://java.sun.com/dtd/properties.dtd">
<properties>
	<comment>IK Analyzer 扩展配置</comment>
	<entry key="ext_dict">custom/mydict.dic;custom/nested</entry>
	<entry key="ext_stopwords"></entry>
	<!-- <entry key="remote_ext_dict">words_location</entry> -->
</properties>`

// The shipped config file carries a byte order mark and a document type
// declaration; the parser has to accept the first and not fetch the second.
func TestLoadFromXMLAcceptsTheShippedConfigShape(t *testing.T) {
	props, err := LoadFromXML(strings.NewReader(bomPrefixedConfig))
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := props.Get("ext_dict"); !ok || got != "custom/mydict.dic;custom/nested" {
		t.Errorf("ext_dict = %q, %t", got, ok)
	}
	if got, ok := props.Get("ext_stopwords"); !ok || got != "" {
		t.Errorf("ext_stopwords = %q, %t; want empty and present", got, ok)
	}
	// A commented-out entry is not an entry.
	if _, ok := props.Get("remote_ext_dict"); ok {
		t.Error("remote_ext_dict should be absent")
	}
}

func TestLoadFromXMLRejectsDocumentsTheDTDWouldNot(t *testing.T) {
	cases := map[string]string{
		"wrong root":        `<config><entry key="a">b</entry></config>`,
		"entry without key": `<properties><entry>b</entry></properties>`,
		"not xml":           `ext_dict=custom/mydict.dic`,
	}
	for name, doc := range cases {
		if _, err := LoadFromXML(strings.NewReader(doc)); err == nil {
			t.Errorf("%s: expected an error", name)
		}
	}
}

func TestLoadFromXMLReportsAMissingKeyDistinctly(t *testing.T) {
	_, err := LoadFromXML(strings.NewReader(`<properties><entry>b</entry></properties>`))
	if !errors.Is(err, ErrInvalidPropertiesFormat) {
		t.Errorf("err = %v, want %v", err, ErrInvalidPropertiesFormat)
	}
}

func TestGetReportsAbsence(t *testing.T) {
	props := Properties{"a": ""}
	if value, ok := props.Get("a"); !ok || value != "" {
		t.Errorf("Get(a) = %q, %t", value, ok)
	}
	if _, ok := props.Get("b"); ok {
		t.Error("Get(b) should report absence")
	}
}
