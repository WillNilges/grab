package main

import (
	"github.com/uptrace/bun"
	"fmt"
	"context"
)

type Instance struct {
		SlackTeamID []byte
		SlackAccessToken []byte
		MediaWikiURL []byte
		MediaWikiUname []byte
		MediaWikiPword []byte
		MediaWikiDomain []byte
}

// Check if we need to initialize the database, and do so if that's the case
func initDB(db *bun.DB) (err error) {
	ctx := context.Background()
	err = db.ResetModel(ctx, &Instance{})
	if err != nil {
		return err
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