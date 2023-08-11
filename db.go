package main

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type Instance struct {
	GrabID           int
	SlackTeamID      []byte
	SlackAccessToken []byte
	MediaWikiURL     []byte
	MediaWikiUname   []byte
	MediaWikiPword   []byte
	MediaWikiDomain  []byte
}

// Check if we need to initialize the database, and do so if that's the case
func initDB(db *bun.DB) (err error) {
	ctx := context.Background()

	instance := new(Instance)
	/*_, err = db.NewDropTable().Model(instance).IfExists().Exec(ctx)
	if err != nil {
		panic(err)
	}*/

	_, err = db.NewCreateTable().Model(instance).IfNotExists().Exec(ctx)
	if err != nil {
		panic(err)
	}

	fmt.Println("Chom")
	chom := new(Instance)
	chom.SlackTeamID = []byte("hello1")
	chom.SlackAccessToken = []byte("hello2")
	chom.MediaWikiURL = []byte("hello3")
	chom.MediaWikiUname = []byte("hello4")
	chom.MediaWikiDomain = []byte("hello5")
	res, err := db.NewInsert().Model(chom).Exec(ctx)
	if err != nil {
		return err
	}
	fmt.Println(res)
	skz := new(Instance)
	err = db.NewSelect().
		Model(skz).
		Scan(ctx)
	if err != nil {
		return err
	}

	fmt.Println(string(skz.SlackTeamID))

	return nil
}
