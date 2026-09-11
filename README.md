IK Analysis for Elasticsearch and OpenSearch
==================================

![](./assets/banner.png)
[![Test](https://github.com/infinilabs/analysis-ik/actions/workflows/test.yml/badge.svg)](https://github.com/infinilabs/analysis-ik/actions/workflows/test.yml)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](LICENSE.txt)

IK Analysis is a Chinese and CJK text analyzer with a customizable dictionary. Maintained and supported with ❤️ by [INFINI Labs](https://infinilabs.com).

It comprises analyzer: `ik_smart` , `ik_max_word`, and tokenizer: `ik_smart` , `ik_max_word`

# How to Install

The analyzer is a Go module. Add it to your project:

```bash
go get github.com/infinilabs/analysis-ik
```

It has no dependencies outside the Go standard library. The dictionaries in
`config/` are data files: copy them next to your application, or point the
configuration at wherever you keep them.

> The packaged Elasticsearch and OpenSearch plugin bundles are JVM artifacts and
> are not produced by this repository. The analyzer names, settings and
> segmentation behaviour they exposed are all reproduced here.

# Getting Started

Tokenize some text with `ik_smart`:

```go
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/infinilabs/analysis-ik/cfg"
	"github.com/infinilabs/analysis-ik/dic"
	"github.com/infinilabs/analysis-ik/lucene"
)

type configuration struct {
	cfg.Settings
	dir string
}

func (c *configuration) ConfDir() string           { return c.dir }
func (c *configuration) ConfigInPluginDir() string { return c.dir }
func (c *configuration) Path(first string, more ...string) string {
	return filepath.Join(append([]string{first}, more...)...)
}
func (c *configuration) SetUseSmart(useSmart bool) cfg.Configuration {
	c.SetUseSmartFlag(useSmart)
	return c
}
func (c *configuration) SetEnableLowercase(enableLowercase bool) cfg.Configuration {
	c.SetEnableLowercaseFlag(enableLowercase)
	return c
}

func main() {
	conf := &configuration{Settings: cfg.NewSettings(), dir: "config"}
	conf.SetUseSmart(true) // ik_smart; false selects ik_max_word
	dic.Initial(conf)

	analyzer := lucene.NewIKAnalyzer(conf)
	defer analyzer.Close()

	stream := analyzer.TokenStream("content", "中华人民共和国国歌")
	if err := stream.Reset(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	for {
		more, err := stream.IncrementToken()
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return
		}
		if !more {
			break
		}
		fmt.Printf("%s\t[%d,%d)\t%s\n",
			stream.CharTermAttribute(),
			stream.OffsetAttribute().StartOffset(),
			stream.OffsetAttribute().EndOffset(),
			stream.TypeAttribute().Type())
	}
	if err := stream.End(); err != nil {
		fmt.Fprintln(os.Stderr, err)
	}
}
```

```
中华人民共和国	[0,7)	CN_WORD
国歌	[7,9)	CN_WORD
```

# How to Build and Test

```bash
go build ./...
go vet ./...
go test ./...
```

# Dictionary Configuration

Config file `IKAnalyzer.cfg.xml` is read from the configuration directory the
`Configuration` reports, falling back to the `config` directory beside the
executable.

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE properties SYSTEM "http://java.sun.com/dtd/properties.dtd">
<properties>
	<entry key="ext_dict">custom/mydict.dic;custom/single_word_low_freq.dic</entry>
	<entry key="ext_stopwords">custom/ext_stopword.dic</entry>
	<entry key="remote_ext_dict">location</entry>
	<entry key="remote_ext_stopwords">http://xxx.com/xxx.dic</entry>
</properties>
```

## Hot-reload Dictionary

The current plugin supports hot reloading dictionary for IK Analysis, through the configuration mentioned earlier in the IK configuration file.

```xml
	<entry key="remote_ext_dict">location</entry>
	<entry key="remote_ext_stopwords">location</entry>
```

Among which `location` refers to a URL, such as `http://yoursite.com/getCustomDict`. This request only needs to meet the following two points to complete the segmentation hot update.

1. The HTTP request needs to return two headers, one is `Last-Modified`, and the other is `ETag`. Both of these are of string type, and if either changes, the plugin will fetch new segmentation to update the word library.

2. The content format returned by the HTTP request is one word per line, and the newline character is represented by `\n`.

Meeting the above two requirements can achieve hot word updates without the need to restart the application.

You can place the hot words that need to be automatically updated in a .txt file encoded in UTF-8. Place it under nginx or another simple HTTP server. When the .txt file is modified, the HTTP server will automatically return the corresponding Last-Modified and ETag when the client requests the file. You can also create a separate tool to extract relevant vocabulary from the business system and update this .txt file.

## FAQs
-------

1. Why isn't the custom dictionary taking effect?

Please ensure that the text format of your custom dictionary is UTF8 encoded.

2. What is the difference between ik_max_word and ik_smart?

ik_max_word: Performs the finest-grained segmentation of the text. For example, it will segment "中华人民共和国国歌" into "中华人民共和国,中华人民,中华,华人,人民共和国,人民,人,民,共和国,共和,和,国国,国歌", exhaustively generating various possible combinations, suitable for Term Query.

ik_smart: Performs the coarsest-grained segmentation of the text. For example, it will segment "中华人民共和国国歌" into "中华人民共和国,国歌", suitable for Phrase queries.

Note: ik_smart is not a subset of ik_max_word.

# Community

Fell free to join the Discord server to discuss anything around this project: 

[https://discord.gg/4tKTMkkvVX](https://discord.gg/4tKTMkkvVX)

# License

Copyright ©️ INFINI Labs.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
