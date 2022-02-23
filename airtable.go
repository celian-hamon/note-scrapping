package main

import (
	"fmt"

	"github.com/mehanizm/airtable"
)

type note struct {
	Name string
	Note string
}

func checkIfExist(Name string) string {
	airtableClient := airtable.NewClient(config.AirtableConfig.ApiKey)
	table := airtableClient.GetTable(config.AirtableConfig.BaseId, config.AirtableConfig.TableName)
	records, err := table.GetRecords().FromView("default").Do()

	if err != nil {
		fmt.Println("Error:", err)
	}

	for _, record := range records.Records {
		if record.Fields["Name"] == Name {
			return "true"
		}
	}
	return "false"
}

func getNotes(name string) []note {
	notes := []note{}
	airtableClient := airtable.NewClient(config.AirtableConfig.ApiKey)
	table := airtableClient.GetTable(config.AirtableConfig.BaseId, config.AirtableConfig.TableName)
	records, err := table.GetRecords().WithFilterFormula("{Name}='" + name + "'").FromView("default").Do()

	if err != nil {
		fmt.Println("Error:", err)
	}

	for _, record := range records.Records {
		//record.Fields
		for noteNom, noteValeur := range record.Fields {
			if noteNom != "Name" {
				str, ok := noteValeur.(string)
				if ok {
					note := note{
						Name: noteNom,
						Note: str,
					}
					notes = append(notes, note)
				} else {
					note := note{
						Name: noteNom,
						Note: noteValeur.([]interface{})[0].(string),
					}
					notes = append(notes, note)
				}
			} else {
				continue
			}
		}
	}
	notes = sortNotes(notes)
	return notes
}

func reloadCache() {
	// airtableClient := airtable.NewClient(config.AirtableConfig.ApiKey)
	// table := airtableClient.GetTable(config.AirtableConfig.BaseId, config.AirtableConfig.TableName)
	// records, _ := table.GetRecords().FromView("default").Do()

}

//use the sample list in the config to sort the notes
func sortNotes(liste []note) []note {
	var sortedList []note
	for _, v := range config.SortingOrder {
		for _, v2 := range liste {
			if v == v2.Name {
				sortedList = append(sortedList, v2)
			}
		}
	}
	return sortedList
}
