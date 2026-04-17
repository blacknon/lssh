# Transform

[![GoDoc](https://img.shields.io/badge/api-reference-blue.svg?style=flat-square)](https://godoc.org/github.com/tidwall/transform)


Transform is a Go package that provides a simple pattern for performing [chainable](#chaining) data transformations on streams of bytes. It conforms to the [io.Reader](https://golang.org/pkg/io/#Reader) interface and is useful for operations such as converting data formats, audio/video resampling, image transforms, log filters, regex line matching, etc.

The [transutil package](#transutil-package) provides few examples that work with JSON such as `JSONToMsgPack`, `MsgPackToJSON`, `JSONToPrettyJSON`, `JSONToUglyJSON`, `JSONToProtoBuf`, and `ProtoBufToJSON`. It also includes a handy `Gzipper` and `Gunzipper`.


Getting Started
===============

## Installing

To start using Transform, install Go and run `go get`:

```sh
$ go get -u github.com/tidwall/transform
```

## Using

Below are a few very simple examples of custom transformers.

### ToUpper

Convert a string to uppper case. Unicode aware. In this example
we only process one rune at a time.

```go
func ToUpper(r io.Reader) io.Reader {
	br := bufio.NewReader(r)
	return transform.NewTransformer(func() ([]byte, error) {
		c, _, err := br.ReadRune()
		if err != nil {
			return nil, err
		}
		return []byte(strings.ToUpper(string([]rune{c}))), nil
	})
}
```
```go
msg := "Hello World"
data, err := ioutil.ReadAll(ToUpper(bytes.NewBufferString(msg)))
if err != nil {
	log.Fatal(err)
}
fmt.Println(string(data))
```

Output:

```
HELLO WORLD
```


### Rot13

The [Rot13](https://en.wikipedia.org/wiki/ROT13) cipher.

```go
func Rot13(r io.Reader) io.Reader {
	buf := make([]byte, 256)
	return transform.NewTransformer(func() ([]byte, error) {
		n, err := r.Read(buf)
		if err != nil {
			return nil, err
		}
		for i := 0; i < n; i++ {
			if buf[i] >= 'a' && buf[i] <= 'z' {
				buf[i] = ((buf[i] - 'a' + 13) % 26) + 'a'
			} else if buf[i] >= 'A' && buf[i] <= 'Z' {
				buf[i] = ((buf[i] - 'A' + 13) % 26) + 'A'
			}
		}
		return buf[:n], nil
	})
}
```
```go
msg := "Hello World"
data, err := ioutil.ReadAll(Rot13(bytes.NewBufferString(msg)))
if err != nil {
	log.Fatal(err)
}
fmt.Println(string(data))
```

Output:

```
Uryyb Jbeyq
```

### RegExp Line Matcher

A line reader that filters lines that match on a RegExp pattern.

```go
func LineMatch(r io.Reader, pattern string) io.Reader {
	br := bufio.NewReader(r)
	return NewTransformer(func() ([]byte, error) {
		for {
			line, err := br.ReadBytes('\n')
			matched, _ := regexp.Match(pattern, line)
			if matched {
				return line, err
			}
			if err != nil {
				return nil, err
			}
		}
	})
}
```
```go
logs := `
23 Apr 17:32:23.604 [INFO] DB loaded in 0.551 seconds
23 Apr 17:32:23.605 [WARN] Disk space is low
23 Apr 17:32:23.054 [INFO] Server started on port 7812
23 Apr 17:32:23.141 [INFO] Ready for connections
`
data, err := ioutil.ReadAll(LineMatch(bytes.NewBufferString(logs), "WARN"))
if err != nil {
	log.Fatal(err)
}
fmt.Println(string(data))
```

Output:

```
23 Apr 17:32:23.605 [WARN] Disk space is low
```

### LineTrimSpace

A line reader that trims the spaces from all lines.

```go
func LineTrimSpace(r io.Reader, pattern string) io.Reader {
	br := bufio.NewReader(r)
	return transform.NewTransformer(func() ([]byte, error) {
		for {
			line, err := br.ReadBytes('\n')
			if len(line) > 0 {
				line = append(bytes.TrimSpace(line), '\n')
			}
			return line, err
		}
	})
}
```
```go
phrases := "  lacy timber \n"
phrases += "\t\thybrid gossiping\t\n"
phrases += " coy radioactivity\n"
phrases += "rocky arrow  \n"
out, err := ioutil.ReadAll(LineTrimSpace(bytes.NewBufferString(phrases)))
if err != nil {
	log.Fatal(err)
}
fmt.Printf("%s\n", out)
```

Output:

```
lacy timber
hybrid gossiping
coy radioactivity
rocky arrow
```

### Chaining

A reader that matches lines on the letter 'o', trims the
space from the lines, and transforms everything to upper case.

```go
phrases := "  lacy timber \n"
phrases += "\t\thybrid gossiping\t\n"
phrases += " coy radioactivity\n"
phrases += "rocky arrow  \n"

r := ToUpper(LineTrimSpace(LineMatch(bytes.NewBufferString(phrases), "o")))

// Pass the string though the transformer.
out, err := ioutil.ReadAll(r)
if err != nil {
	log.Fatal(err)
}

fmt.Printf("%s\n", out)
```

Output:

```
HYBRID GOSSIPING
COY RADIOACTIVITY
ROCKY ARROW
```

## Transutil package
[![GoDoc](https://img.shields.io/badge/api-reference-blue.svg?style=flat-square)](https://godoc.org/github.com/tidwall/transform/transutil)


The `github.com/tidwall/transform/transutil` package includes additional examples.

```
func Gunzipper(r io.Reader) io.Reader
func Gzipper(r io.Reader) io.Reader
func JSONToMsgPack(r io.Reader) io.Reader
func JSONToPrettyJSON(r io.Reader) io.Reader
func JSONToProtoBuf(r io.Reader, pb proto.Message, multimessage bool) io.Reader
func JSONToUglyJSON(r io.Reader) io.Reader
func MsgPackToJSON(r io.Reader) io.Reader
func ProtoBufToJSON(r io.Reader, pb proto.Message, multimessage bool) io.Reader
```

## Contact
Josh Baker [@tidwall](http://twitter.com/tidwall)

## License
Transform source code is available under the ISC [License](/LICENSE).


