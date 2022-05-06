# JSONJ

JSONJ can be used to manipulate raw json input using _marks_ and custom _fragments generators_.
* Library guarantees valid json output syntax;
* Library doesn't validate output json semantic like unique keys.

## Marks

One can apply generator to  _marks_ of json input. Each _mark_ is json key name.
For example, `uuid` and `id` maybe used as _mark_.
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

_Mark_ maybe renamed in result of _operation_.  It depends on `operation` mode and its rules.

Advice: wrap _marks_ by special chars, i.e. `__uuid__` and unwrap during `operation`.


## Operations
Library supports number of operations, named _Mode_:
  * `ModeInsert`: insert key/value pair after the _mark_.
  * `ModeReplaceValue`: replace value, or convert it;
  * `ModeReplace`: replace entire key/value pair;
  * `ModeDelete`: delete key/value.

## Fragments generators

Type `GenerateFragmentBatchFunc` describes interface of generators.
Key feature of generators is batch processing. Batches speed up result output.

Example:
```go
// GeneratorParams customize generator behavior
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
