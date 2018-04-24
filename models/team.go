package models

type Team struct {
	ID       string `bson:"_id"`
	Name     string `bson:"name"`
	Domain   string `bson:"domain"`
	Token    string `bson:"token"`
	Disabled bool   `bson:"disabled"`
}
