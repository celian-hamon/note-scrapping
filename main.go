package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	//Discord
	"github.com/bwmarrin/discordgo"

	//Database
	"database/sql"

	_ "github.com/go-sql-driver/mysql"
)

/*
TODO - add cache
TODO - add note from student sharing
*/

type Config struct {
	AirtableConfig AirtableConfig `json:"airtableConf"`
	DiscordConfig  DiscordConfig  `json:"discordConf"`
	DatabaseConfig DatabaseConfig `json:"databaseConf"`
	SortingOrder   []string       `json:"sortingOrder"`
}

type AirtableConfig struct {
	ApiKey    string `json:"token"`
	BaseId    string `json:"baseId"`
	TableName string `json:"tableName"`
}

type DiscordConfig struct {
	Token string `json:"token"`
}

type DatabaseConfig struct {
	User     string `json:"user"`
	Password string `json:"password"`
	Host     string `json:"host"`
	Port     string `json:"port"`
	Database string `json:"database"`
}
type User struct {
	Name      string `json:"name"`
	DiscordId string `json:"discordId"`
	Sharing   int    `json:"sharing"`
}

func loadConfig() {
	file, err := os.Open("config.json")
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		fmt.Println("Error:", err)
		os.Exit(1)
	}
}

var config Config

func main() {
	loadConfig()
	// Create a new Discord session using the provided bot token.
	dg, err := discordgo.New("Bot " + config.DiscordConfig.Token)
	if err != nil {
		fmt.Println("Error creating Discord session: ", err)
		return
	}

	// Register ready as a callback for the ready events.
	dg.AddHandler(ready)

	// Register messageCreate as a callback for the messageCreate events.
	dg.AddHandler(messageCreate)

	// We need information about guilds (which includes their channels),
	// messages and voice states.
	dg.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMessages

	// Open the websocket and begin listening.
	err = dg.Open()
	if err != nil {
		fmt.Println("Error opening Discord session: ", err)
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Pornote is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

// This function will be called (due to AddHandler above) when the bot receives
// the "ready" event from Discord.
func ready(s *discordgo.Session, event *discordgo.Ready) {

	reloadCache()
	// Set the playing status.
	s.UpdateGameStatus(0, "dev env")
}

func messageCreate(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Message.Content == "!me" {
		db, err := sql.Open("mysql", config.DatabaseConfig.User+":"+config.DatabaseConfig.Password+"@tcp("+config.DatabaseConfig.Host+":"+config.DatabaseConfig.Port+")/"+config.DatabaseConfig.Database)
		if err != nil {
			fmt.Println("Error connecting:", err)
			return
		}
		defer db.Close()

		results, err := db.Query("SELECT name FROM students WHERE discordId = ?", m.Author.ID)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		defer results.Close()

		var name string

		for results.Next() {
			err = results.Scan(&name)
			if err != nil {
				fmt.Println("Error:", err)
				return
			}
		}
		if name == "" {
			s.ChannelMessageSend(m.ChannelID, "You are not registered yet.")
		} else {
			s.ChannelMessageSend(m.ChannelID, "Your name is "+name)
		}

	}

	if strings.HasPrefix(m.Message.Content, "!register") {
		NAME := m.Message.Content[10:]
		if NAME == "" {
			s.ChannelMessageSend(m.ChannelID, "Please enter a name.")
			return
		}
		loadingBar, _ := s.ChannelMessageSend(m.ChannelID, "██░░░░░░")
		s.ChannelMessageSend(m.ChannelID, "Your name is "+NAME)

		db, err := sql.Open("mysql", config.DatabaseConfig.User+":"+config.DatabaseConfig.Password+"@tcp("+config.DatabaseConfig.Host+":"+config.DatabaseConfig.Port+")/"+config.DatabaseConfig.Database)
		if err != nil {
			fmt.Println("Error connecting:", err)
			return
		}
		defer db.Close()
		s.ChannelMessageSend(m.ChannelID, "Successfully connected to db")
		s.ChannelMessageEdit(m.ChannelID, loadingBar.ID, "████░░░░")

		results, err := db.Query("SELECT name FROM students WHERE discordId = ?", m.Author.ID)
		if err != nil {
			fmt.Println("Error:", err)
			return
		}
		s.ChannelMessageSend(m.ChannelID, "Checking if you are already registered")
		s.ChannelMessageEdit(m.ChannelID, loadingBar.ID, "██████░░")

		var name string

		for results.Next() {
			err = results.Scan(&name)
			if err != nil {
				fmt.Println("Error:", err)
				return
			}
		}
		defer results.Close()
		if name == "" {
			if checkIfExist(NAME) == "true" {
				// perform a db.Query insert
				insert, err := db.Query("INSERT INTO students (discordId,name) VALUES ( ?, ? )", m.Author.ID, NAME)

				// if there is an error inserting, handle it
				if err != nil {
					panic(err.Error())
				}
				insert.Close()
				s.ChannelMessageSendReply(m.ChannelID, "You are now registered.", m.MessageReference)
				s.ChannelMessageEdit(m.ChannelID, loadingBar.ID, "████████")
				return
			} else {
				s.ChannelMessageSend(m.ChannelID, "That name isnt on airtable.")
				return
			}

		} else {
			s.ChannelMessageSendReply(m.ChannelID, "You are already registered.", m.MessageReference)
			s.ChannelMessageEdit(m.ChannelID, loadingBar.ID, "██████░░")
			return
		}
	}
	if strings.HasPrefix(m.Message.Content, "!mynotes") {
		loadingBar, _ := s.ChannelMessageSend(m.ChannelID, "██░░░░░░")
		state := isRegistered(m.Author.ID)

		if state == "false" {
			s.ChannelMessageSend(m.ChannelID, "You are not registered yet.")
		} else {
			s.ChannelMessageEdit(m.ChannelID, loadingBar.ID, "████░░░░")
			embed := &discordgo.MessageEmbed{
				Title:  "Your notes",
				Color:  0x00ff00,
				Fields: []*discordgo.MessageEmbedField{},
			}
			notes := getNotes(state)
			s.ChannelMessageEdit(m.ChannelID, loadingBar.ID, "██████░░")

			for _, note := range notes {

				embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
					Name:   note.Name,
					Value:  note.Note,
					Inline: false,
				})
			}
			s.ChannelMessageEdit(m.ChannelID, loadingBar.ID, "████████")
			s.ChannelMessageSendEmbed(m.ChannelID, embed)
		}
	}
	if strings.HasPrefix(m.Message.Content, "!notes") {
		loadingBar, _ := s.ChannelMessageSend(m.ChannelID, "██░░░░░░")
		state := isRegistered(m.Author.ID)

		if state == "false" {
			s.ChannelMessageSend(m.ChannelID, "You are not registered yet.")
		} else {
			NAME := m.Message.Content[7:]
			var user User
			if len(m.Mentions) > 0 {
				user = getUser("#" + m.Mentions[0].ID)
			} else if NAME != "" {
				user = getUser(NAME)
			} else {
				s.ChannelMessageSend(m.ChannelID, "Please enter a name.")
			}
			if user.DiscordId == "" {
				s.ChannelMessageSend(m.ChannelID, "That user isnt registered.")
				return
			}
			if user.Sharing == 0 {
				s.ChannelMessageSend(m.ChannelID, "That user aint sharing.")
				return
			}

			s.ChannelMessageEdit(m.ChannelID, loadingBar.ID, "████░░░░")
			embed := &discordgo.MessageEmbed{
				Title:  "Notes for " + user.Name,
				Color:  0x00ff00,
				Fields: []*discordgo.MessageEmbedField{},
			}
			notes := getNotes(user.Name)
			s.ChannelMessageEdit(m.ChannelID, loadingBar.ID, "██████░░")

			for _, note := range notes {

				embed.Fields = append(embed.Fields, &discordgo.MessageEmbedField{
					Name:   note.Name,
					Value:  note.Note,
					Inline: false,
				})
			}
			s.ChannelMessageEdit(m.ChannelID, loadingBar.ID, "████████")
			s.ChannelMessageDelete(m.ChannelID, loadingBar.ID)
			s.ChannelMessageSendEmbed(m.ChannelID, embed)
		}
	}

	if strings.HasPrefix(m.Message.Content, "!share") {
		state := isRegistered(m.Author.ID)
		if state != "false" {
			boolean := m.Message.Content[7:]
			db, err := sql.Open("mysql", config.DatabaseConfig.User+":"+config.DatabaseConfig.Password+"@tcp("+config.DatabaseConfig.Host+":"+config.DatabaseConfig.Port+")/"+config.DatabaseConfig.Database)
			if err != nil {
				fmt.Println("Error connecting:", err)
				return
			}
			defer db.Close()

			q, _ := db.Query("SELECT sharing FROM students WHERE discordId = ?", m.Author.ID)
			defer q.Close()

			var share string

			for q.Next() {
				err = q.Scan(&share)
				if err != nil {
					fmt.Println("Error:", err)
					return
				}
			}
			if share != boolean {

				update, err := db.Query("UPDATE students SET sharing = ? WHERE discordId = ?", boolean, m.Author.ID)

				// if there is an error inserting, handle it
				if err != nil {
					panic(err.Error())
				}
				update.Close()

				if boolean == "0" {

					s.ChannelMessageSend(m.ChannelID, "You have disabled sharing your notes.")
					return
				} else if boolean == "1" {
					s.ChannelMessageSend(m.ChannelID, "You have enabled sharing your notes.")
					return
				}
			} else {
				s.ChannelMessageSend(m.ChannelID, "You are already on this state.")
				return
			}
		} else {
			s.ChannelMessageSend(m.ChannelID, "You are not registered yet.")
			return
		}

	}
}

func isRegistered(id string) string {
	db, err := sql.Open("mysql", config.DatabaseConfig.User+":"+config.DatabaseConfig.Password+"@tcp("+config.DatabaseConfig.Host+":"+config.DatabaseConfig.Port+")/"+config.DatabaseConfig.Database)
	if err != nil {
		fmt.Println("Error connecting:", err)
		return "false"
	}
	defer db.Close()

	results, err := db.Query("SELECT name FROM students WHERE discordId = ?", id)
	if err != nil {
		fmt.Println("Error:", err)
		return "false"
	}
	defer results.Close()

	var name string

	for results.Next() {
		err = results.Scan(&name)
		if err != nil {
			fmt.Println("Error:", err)
			return "false"
		}
	}
	if name == "" {
		return "false"
	} else {
		return name
	}
}

func getUser(input string) User {
	db, err := sql.Open("mysql", config.DatabaseConfig.User+":"+config.DatabaseConfig.Password+"@tcp("+config.DatabaseConfig.Host+":"+config.DatabaseConfig.Port+")/"+config.DatabaseConfig.Database)
	if err != nil {
		fmt.Println("Error connecting:", err)
	}
	defer db.Close()

	var user User

	if input[0] == '#' {
		input = input[1:]
		fmt.Sprintf("%+v", input)
		results, err := db.Query("SELECT name, discordId,sharing FROM students WHERE discordId = ?", input)
		if err != nil {
			fmt.Println("Error:", err)
		}
		defer results.Close()
		for results.Next() {
			err = results.Scan(&user.Name, &user.DiscordId, &user.Sharing)
			if err != nil {
				fmt.Println("Error:", err)
			}
		}
		return user

	} else {
		results, err := db.Query("SELECT name, discordId,sharing FROM students WHERE name = ?", input)
		if err != nil {
			fmt.Println("Error:", err)
		}
		defer results.Close()
		for results.Next() {
			err = results.Scan(&user.Name, &user.DiscordId, &user.Sharing)
			if err != nil {
				fmt.Println("Error:", err)
			}
		}
		return user

	}

}
