package importer

import mgo "gopkg.in/mgo.v2"

type Database struct {
	*mgo.Database
	Channels *mgo.Collection
	Teams    *mgo.Collection
	Users    *mgo.Collection
	Messages *mgo.Collection
}

// TODO: separate db  and session, make db configurable
func (d *Database) Open(dsn string) error {
	session, err := mgo.Dial(dsn)
	if err != nil {
		return err
	}

	session.SetMode(mgo.Monotonic, true)

	d.Database = session.DB("slackarchive")

	d.Teams = d.Database.C("teams")
	d.Users = d.Database.C("users")
	d.Channels = d.Database.C("channels")
	d.Messages = d.Database.C("messages")
	return nil
}
