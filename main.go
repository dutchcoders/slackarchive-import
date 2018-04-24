package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sync"

	"github.com/dutchcoders/slackarchive-import/config"
	"github.com/dutchcoders/slackarchive-import/models"
	"github.com/dutchcoders/slackarchive-import/utils"

	"github.com/nlopes/slack"

	logging "github.com/op/go-logging"
	mgo "gopkg.in/mgo.v2"
	elastic "gopkg.in/olivere/elastic.v5"
)

var log = logging.MustGetLogger("example")

func New(conf *config.Config) *Importer {
	db := Database{}
	if err := db.Open(conf.DSN); err != nil {
		log.Error("Error opening database: %s", err.Error())
		return nil
	}

	es, err := elastic.NewClient(elastic.SetURL(conf.ElasticSearch.Host), elastic.SetSniff(false))
	if err != nil {
		panic(err)
	}

	return &Importer{
		db: &db,
		es: es,
	}
}

type Importer struct {
	db *Database
	es *elastic.Client

	conf *config.Config
}

type TeamImporter struct {
	*Importer

	client   *slack.Client
	team     *models.Team
	channels map[string]models.Channel
}

func (i *Importer) importTeam(client *slack.Client, token string) (*models.Team, error) {
	team, err := client.GetTeamInfo()
	if err != nil {
		return nil, err
	}

	t := models.Team{}
	if err := i.db.Teams.FindId(team.ID).One(&t); err == nil {
		log.Debug("Team already exists: %s", t.ID)
	} else {
		return nil, err
	}

	if err := utils.Merge(&t, *team); err != nil {
		return nil, err
	}

	if err = i.db.Teams.Insert(&t); err == nil {
	} else if mgo.IsDup(err) {
		log.Info(err.Error())
	} else {
		return nil, err
	}

	return &t, nil
}

func (i *TeamImporter) importUsers(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}

	defer f.Close()

	var users []slack.User
	if err := json.NewDecoder(f).Decode(&users); err != nil {
		return err
	}

	for _, user := range users {
		u := models.User{}
		if err := i.db.Users.FindId(user.ID).One(&u); err == nil {
			log.Debug("User already exists: %s", u.ID)
			continue
		} else {

			return err
		}

		if err := utils.Merge(&u, user); err != nil {
			log.Error("Error merging user(%s): %s", user.ID, err.Error())
			continue
		}

		u.Team = i.team.ID
		if err = i.db.Users.Insert(&u); err == nil {
		} else if mgo.IsDup(err) {
			log.Info(err.Error())
		} else {
			log.Error("Error upserting user(%s): %s", user.ID, err.Error())
			continue
		}
	}

	return nil
}

func (i *TeamImporter) importChannels(path string) (map[string]models.Channel, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f.Close()

	channelsMap := map[string]models.Channel{}

	var channels []slack.Channel
	if err := json.NewDecoder(f).Decode(&channels); err != nil {
		return nil, err
	}

	for _, channel := range channels {
		c := models.Channel{}

		if err := i.db.Channels.FindId(channel.ID).One(&c); err == nil {
			channelsMap[c.Name] = c

			log.Debug("Channel already exists: %s", c.ID)
			// found
			continue
		} else {
			// if not found
		}

		if err := utils.Merge(&c, channel); err != nil {
			log.Error("Error merging channel(%s): %s", channel.ID, err.Error())
			continue
		}

		c.Team = i.team.ID

		if err = i.db.Channels.Insert(&c); err == nil {
		} else if mgo.IsDup(err) {
			log.Info(err.Error())
		} else {
			log.Error("Error inserting channel(%s): %s", channel.ID, err.Error())
			continue
		}

		channelsMap[c.Name] = c
	}

	i.channels = channelsMap

	return channelsMap, nil
}

func (ti *TeamImporter) importMessages(channelID string, path string) error {

	var wg sync.WaitGroup

	mongoInserter := make(chan models.Message)

	wg.Add(1)
	go func() {
		defer wg.Done()

		bulk := ti.db.Messages.Bulk()
		defer bulk.Run()

		for msg := range mongoInserter {
			bulk.Insert(msg)
		}
	}()

	elasticInserter := make(chan models.Message)

	wg.Add(1)
	go func() {
		defer wg.Done()

		bulk := ti.es.Bulk()

		count := 0

		index := func() {
			if bulk.NumberOfActions() == 0 {
			} else if response, err := bulk.Do(context.Background()); err != nil {
				log.Error("Error indexing: ", err.Error())
			} else {
				indexed := response.Indexed()
				count += len(indexed)

				log.Info("Bulk indexing: %d total %d.", len(indexed), count)
			}
		}

		for msg := range elasticInserter {
			bulk = bulk.Add(elastic.NewBulkIndexRequest().
				Index(fmt.Sprintf("slackarchive")).
				Type("message").
				Id(msg.ID).
				Doc(msg),
			)
		}

		index()
	}()

	f, err := os.Open(path)
	if err != nil {
		return err
	}

	defer f.Close()

	var messages []slack.Message
	if err := json.NewDecoder(f).Decode(&messages); err != nil {
		return err
	}

	for _, message := range messages {
		m := models.Message{}
		if err := utils.Merge(&m, message); err != nil {
			log.Error("Error merging message: %s", err.Error())
			continue
		}

		m.ID = fmt.Sprintf("%s-%s-%s", ti.team.ID, channelID, m.Timestamp)
		m.Team = ti.team.ID
		m.Channel = channelID

		if err := ti.db.Messages.FindId(m.ID).One(&m); err == nil {
			//	log.Debug("Message already exists: %s", m.ID)
			//fmt.Printf("%#v\n", m)
		} else {
			//	fmt.Printf("%#v\n", m)
		}

		elasticInserter <- m
		mongoInserter <- m
	}

	close(elasticInserter)
	close(mongoInserter)

	wg.Wait()

	return nil
}

func (i *Importer) Import(token string, importPath string) (*TeamImporter, error) {
	client := slack.New(token)

	team, err := i.importTeam(client, token)
	if err != nil {
		log.Error("GetTeamInfo: %s", err.Error())
		return nil, err
	}

	ti := &TeamImporter{
		Importer: i,

		client:   client,
		team:     team,
		channels: map[string]models.Channel{},
	}

	importPath = path.Join(importPath, team.Domain)

	if err := ti.importUsers(path.Join(importPath, "users.json")); err != nil {
		log.Error("importUsers: %s", err.Error())
	}

	if _, err := ti.importChannels(path.Join(importPath, "channels.json")); err != nil {
		log.Error("importChannel: %s", err.Error())
	}

	for _, channel := range ti.channels {
		channelPath := path.Join(importPath, channel.Name)
		filepath.Walk(channelPath, func(path string, info os.FileInfo, err error) error {
			if info.IsDir() {
				return nil
			}

			log.Info("Importing path: %s", path)

			if err := ti.importMessages(channel.ID, path); err != nil {
				log.Error("importMessages: %s", err.Error())
			}

			return nil
		})
	}

	return ti, nil
}

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: import {token} {path}")
		return
	}

	token := os.Args[1]
	p := os.Args[2]

	conf := &config.Config{}
	conf.Load("config.yaml")

	importer := New(conf)
	importer.Import(token, p)
}
