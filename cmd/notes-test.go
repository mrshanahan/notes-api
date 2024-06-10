package main

// import (
// 	"fmt"

// 	"github.com/mrshanahan/notes-api/pkg/client"
// 	"github.com/mrshanahan/notes-api/pkg/notes"
// )

// func main() {
// 	client := client.NewClient("http://localhost:4444/")
// 	// note, err := client.GetNote(40)
// 	// if err != nil {
// 	//     panic(err)
// 	// }
// 	// printNote(note)
// 	notes, err := client.ListNotes()
// 	if err != nil {
// 		panic(err)
// 	}
// 	for _, n := range notes {
// 		printNote(n)
// 	}

// 	fmt.Println("--------------")

// 	note, err := client.CreateNote("this is a new note")
// 	if err != nil {
// 		panic(err)
// 	}
// 	fmt.Println("New note:")
// 	printNote(note)

// 	fmt.Println("--------------")

// 	notes, err = client.ListNotes()
// 	if err != nil {
// 		panic(err)
// 	}
// 	for _, n := range notes {
// 		printNote(n)
// 	}

// 	fmt.Println("--------------")

// 	note, err = client.GetNote(note.ID)
// 	if err != nil {
// 		panic(err)
// 	}
// 	fmt.Println("GOT new note:")
// 	printNote(note)

// 	fmt.Println("--------------")

// 	err = client.UpdateNoteContent(note.ID, []byte("This is the\nnew content!"))
// 	if err != nil {
// 		panic(err)
// 	}

// 	fmt.Println("Content updated!")

// 	fmt.Println("--------------")

// 	content, err := client.GetNoteContent(note.ID)
// 	fmt.Println("Content for new note:")
// 	fmt.Println(string(content))

// 	fmt.Println("--------------")

// 	err = client.DeleteNote(note.ID)
// 	if err != nil {
// 		panic(err)
// 	}
// 	fmt.Println("Deleted!")

// 	fmt.Println("--------------")

// 	notes, err = client.ListNotes()
// 	if err != nil {
// 		panic(err)
// 	}
// 	for _, n := range notes {
// 		printNote(n)
// 	}
// }

// func printNote(n *notes.Note) {
// 	fmt.Printf("[%d] %s (created: %s, updated: %s)\n", n.ID, n.Title, n.CreatedOn, n.UpdatedOn)
// }
