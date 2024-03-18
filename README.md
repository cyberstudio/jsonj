# JSONJ

[![Go](https://github.com/cyberstudio/jsonj/actions/workflows/go.yml/badge.svg)](https://github.com/cyberstudio/jsonj/actions/workflows/go.yml) [![GoDoc](https://godoc.org/github.com/cyberstudio/jsonj?status.svg)](https://godoc.org/github.com/cyberstudio/jsonj) [![Go Report Card](https://goreportcard.com/badge/github.com/cyberstudio/jsonj)](https://goreportcard.com/report/github.com/cyberstudio/jsonj)

The library seeks for marks in input json and applies fragment generators, producing a new json. 
It works with rules that are combinations or marks and generators.

The library guarantees a valid output json syntax on valid input json. 
Also, it is possible to generate a semantically invalid json if fragment generators are not correct. 
For example, an incorrect fragment generator can produce duplicate object keys.

## Marks

Each _mark_ is an object key name. One can apply generator to _marks_ of input.
For example, `uuid` and `id` can be used as _mark_.
```json
[
    {
        "uuid": "302b7140-dfff-4f4b-9e72-5b731ec14d85",
        "id": 1234
    },
    {
        "uuid": "302b7140-dfff-4f4b-9e72-5b731ec14d85",
        "id": 1234
    }
]
```

_Mark_ can be renamed in result of _operation_.  It depends on `operation` mode and its rules.

Advice: wrap _marks_ in special symbols, i.e. `__uuid__` and unwrap during `operation`.


## Operations

The library supports a number of operations, named _Mode_:
  * `ModeInsert`: insert key/value pair after the _mark_,
  * `ModeReplace`: replace the entire key/value pair,
  * `ModeReplaceValue`: replace or convert value and keep key as is,
  * `ModeDelete`: delete key/value.

## Fragments generators

Implement GenerateFragmentBatchFunc interface to create a custom fragment generator.

Example:
```go
// GeneratorParams customizes generator behavior
type GeneratorParams struct{
    EmbedObjectURL bool
    BaseURL        string
}

// Generator returns batch of "url": "http://localhost/{id}" fragments
// to be inserted to json
func Generator(ctx context.Context, iterator jsonj.FragmentIterator, p interface{}) ([]interface{}, error) {
    params := p.(GeneratorParams)
    if !params.EmbedObjectURL {
        return jsonj.EmptyFragmentsGenerator(ctx, iterator, p)
    }
    type Item struct {
        URL *string `json:"url"`
    }
    result := make([]interface{}, 0, iterator.Count())
    for iterator.Next() {
        var id int64
        if err := iterator.BindParams(&id); err != nil {
            panic(err)
        }
        var item Item
        if id != 0 {
            addr := fmt.Sprintf("%s/%s", params.BaseURL, id)
            item.URL = &addr
        }
        result = append(result, item)
    }
    return result, nil
}
```

Batch processing is a key feature of generators. It speeds up the result output.
