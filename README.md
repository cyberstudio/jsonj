# JSONJ

[![Go](https://github.com/cyberstudio/jsonj/actions/workflows/go.yml/badge.svg)](https://github.com/cyberstudio/jsonj/actions/workflows/go.yml) [![GoDoc](https://godoc.org/github.com/cyberstudio/jsonj?status.svg)](https://godoc.org/github.com/cyberstudio/jsonj) [![Go Report Card](https://goreportcard.com/badge/github.com/cyberstudio/jsonj)](https://goreportcard.com/report/github.com/cyberstudio/jsonj)

JSONJ can be used to manipulate raw json input using _marks_ and custom _fragments generators_.
* Library guarantees a valid json output syntax;
* Library doesn't validate an output json semantic like unique keys.

## Marks

One can apply generator to  _marks_ of json input. Each _mark_ is a json key name.
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
  * `ModeInsert`: insert key/value pair after the _mark_.
  * `ModeReplaceValue`: replace or convert value;
  * `ModeReplace`: replace entire key/value pair;
  * `ModeDelete`: delete key/value.

## Fragments generators

Type `GenerateFragmentBatchFunc` describes an interface of generators.
Batch processing is a key feature of generators. Batches speed up the result output.

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
