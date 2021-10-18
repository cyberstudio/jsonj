package jsonj_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/cyberstudio/jsonj"
)

// Pet is node of family tree
type Pet struct {
	ID       int64   `json:"pet_id"`        // replaced by uuid/url
	FamilyID int64   `json:"pet_family_id"` // replaced by family short view
	Nick     string  `json:"nick"`
	Children []int64 `json:"pet_children"` // replaced by PetRef view
}

type Family struct {
	ID        int64  `json:"family_id"` // replaced by uuid/url
	ShortName string `json:"name"`
	LongName  string `json:"family_long_name"` // removed in short view
}

var passes = []jsonj.Pass{
	{
		RuleSet: jsonj.NewRuleSet(
			// pets children
			jsonj.NewReplaceValueRule("pet_children", "children", petChildren),
		),
		Repeats: 2, // depth of children
	},
	{
		RuleSet: jsonj.NewRuleSet(
			// pet
			jsonj.NewReplaceValueRule("pet_id", "pet_uuid", fetchPetUUID),
			jsonj.NewInsertRule("pet_uuid", "uuid", appendPetURL),
			jsonj.NewReplaceValueRule("pet_family_id", "family", fetchFamily),
			jsonj.NewDeleteRule("pet_children"),
		),
		Repeats: 2, // no less than count of marks name connectivity in RuleSet (pet_id -> pet_uuid -> uuid)
	},
	{
		RuleSet: jsonj.NewRuleSet(
			// family
			jsonj.NewReplaceValueRule("family_id", "family_uuid", replaceFamilyIDs),
			jsonj.NewInsertRule("family_uuid", "uuid", appendFamilyURL),
			jsonj.NewDeleteRule("family_long_name"),
		),
		Repeats: 2, // no less than count of marks name connectivity in RuleSet (family_id -> family_uuid -> uuid)
	},
}

// ProcessParams used to customize Process params
type ProcessParams struct {
	BaseURL string
}

func ExampleProcess() {
	input, err := json.MarshalIndent(petByID[1], "", "  ")
	if err != nil {
		panic(err)
	}
	// Input:
	// {
	//  "pet_id": 1,
	//  "pet_family_id": 9,
	//  "name": "KittyCat",
	//  "pet_children": [
	//    2, 3
	//  ]
	// }

	params := jsonj.ProcessParams{
		Passes: passes,
		Params: &ProcessParams{
			BaseURL: "https://zoo.com",
		},
	}
	output, err := jsonj.Process(context.Background(), input, params)
	if err != nil {
		panic(err)
	}

	var dst bytes.Buffer
	if err := json.Indent(&dst, output, "", "  "); err != nil {
		panic(err)
	}
	fmt.Print(dst.String())
	// Output:
	// {
	//   "uuid": "74ea3f44-ba35-4d2d-8a3e-01fb4c458df4",
	//   "url": "https://zoo.com/pets/74ea3f44-ba35-4d2d-8a3e-01fb4c458df4",
	//   "family": {
	//     "uuid": "fe4188f6-1993-4cce-8726-34294bfd1f1b",
	//     "url": "https://zoo.com/families/fe4188f6-1993-4cce-8726-34294bfd1f1b",
	//     "name": "Cat"
	//   },
	//   "nick": "KittyCat",
	//   "children": [
	//     {
	//       "uuid": "37f5e2bf-979b-47b4-baca-7362fd1e4a4d",
	//       "url": "https://zoo.com/pets/37f5e2bf-979b-47b4-baca-7362fd1e4a4d",
	//       "children": [
	//         {
	//           "uuid": "cde5e21f-e5e1-45aa-9140-6b5512cbd011",
	//           "url": "https://zoo.com/pets/cde5e21f-e5e1-45aa-9140-6b5512cbd011"
	//         },
	//         {
	//           "uuid": "695a18d8-135c-4082-bbfa-e745db730570",
	//           "url": "https://zoo.com/pets/695a18d8-135c-4082-bbfa-e745db730570"
	//         }
	//       ]
	//     },
	//     {
	//       "uuid": "cde5e21f-e5e1-45aa-9140-6b5512cbd011",
	//       "url": "https://zoo.com/pets/cde5e21f-e5e1-45aa-9140-6b5512cbd011",
	//       "children": [
	//         {
	//           "uuid": "695a18d8-135c-4082-bbfa-e745db730570",
	//           "url": "https://zoo.com/pets/695a18d8-135c-4082-bbfa-e745db730570"
	//         }
	//       ]
	//     }
	//   ]
	// }
}

func appendPetURL(ctx context.Context, iterator jsonj.FragmentIterator, p interface{}) ([]interface{}, error) {
	return generateURLs(ctx, iterator, p.(*ProcessParams).BaseURL+"/pets/")
}

func appendFamilyURL(ctx context.Context, iterator jsonj.FragmentIterator, p interface{}) ([]interface{}, error) {
	return generateURLs(ctx, iterator, p.(*ProcessParams).BaseURL+"/families/")
}

func generateURLs(_ context.Context, iterator jsonj.FragmentIterator, urlPrefix string) ([]interface{}, error) {
	type Entity struct {
		URL *string `json:"url"`
	}

	entities := make([]interface{}, 0, iterator.Count())
	for iterator.Next() {
		var (
			id     string
			entity Entity
		)
		if err := iterator.BindParams(&id); err != nil {
			panic(err)
		}

		if id != "" {
			url := urlPrefix + id
			entity.URL = &url
		}
		entities = append(entities, entity)
	}
	return entities, nil
}

func fetchPetUUID(ctx context.Context, iterator jsonj.FragmentIterator, _ interface{}) ([]interface{}, error) {
	return generateUUIDs(ctx, iterator)
}

func replaceFamilyIDs(ctx context.Context, iterator jsonj.FragmentIterator, _ interface{}) ([]interface{}, error) {
	return generateUUIDs(ctx, iterator)
}

func generateUUIDs(_ context.Context, iterator jsonj.FragmentIterator) ([]interface{}, error) {
	uuids := make([]interface{}, 0, iterator.Count())
	for iterator.Next() {
		var id int64
		if err := iterator.BindParams(&id); err != nil {
			panic(err)
		}
		uuids = append(uuids, uuidBySerialID[id])
	}
	return uuids, nil
}

func fetchFamily(_ context.Context, iterator jsonj.FragmentIterator, _ interface{}) ([]interface{}, error) {
	families := make([]interface{}, 0, iterator.Count())
	for iterator.Next() {
		var id int64
		if err := iterator.BindParams(&id); err != nil {
			panic(err)
		}
		families = append(families, familyByID[id])
	}
	return families, nil
}

func petChildren(_ context.Context, iterator jsonj.FragmentIterator, _ interface{}) ([]interface{}, error) {
	type Children struct {
		ID       int64   `json:"pet_id"`
		Children []int64 `json:"pet_children"`
	}
	fragments := make([]interface{}, 0, iterator.Count())
	for iterator.Next() {
		var ids []int64
		if err := iterator.BindParams(&ids); err != nil {
			panic(err)
		}

		children := make([]Children, 0, len(ids))
		for _, id := range ids {
			children = append(children, Children{
				ID:       id,
				Children: petByID[id].Children,
			})
		}
		fragments = append(fragments, children)
	}
	return fragments, nil
}

var (
	// uuidBySerialID emulates storages
	uuidBySerialID = map[int64]string{
		1: `74ea3f44-ba35-4d2d-8a3e-01fb4c458df4`,
		2: `37f5e2bf-979b-47b4-baca-7362fd1e4a4d`,
		3: `cde5e21f-e5e1-45aa-9140-6b5512cbd011`,
		4: `695a18d8-135c-4082-bbfa-e745db730570`,
		9: `fe4188f6-1993-4cce-8726-34294bfd1f1b`,
	}

	// familyByID emulates families storage
	familyByID = map[int64]Family{
		9: {
			ID:        9,
			ShortName: "Cat",
			LongName:  "Felix cat",
		},
	}

	// petByID emulates pets storage
	petByID = map[int64]Pet{
		1: {
			ID:       1,
			FamilyID: 9,
			Nick:     "KittyCat",
			Children: []int64{2, 3},
		},
		2: {
			ID:       2,
			FamilyID: 9,
			Nick:     "PussyCat",
			Children: []int64{3, 4},
		},
		3: {
			ID:       3,
			FamilyID: 9,
			Nick:     "CooperCat",
			Children: []int64{4},
		},
		4: {
			ID:       4,
			FamilyID: 9,
			Nick:     "HerculesCat",
		},
	}
)
