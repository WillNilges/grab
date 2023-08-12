package main

import (
	"context"

	"github.com/uptrace/bun"
)

// An instance of Grab is in reference to an organization using Grab. For now
// they can only have a Slack and a MediaWiki, but I'll probably want to change
// the schema when I start adding more stuff. Might want to make some relations
// and what have you
type Instance struct {
	GrabID           string
	SlackTeamID      string
	SlackAccessToken string
	MediaWikiURL     string
	MediaWikiUname   string
	MediaWikiPword   string
	MediaWikiDomain  string
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

	return nil
}

// Should only return one instance
func selectInstance(db *bun.DB, grabID string) (instance *Instance) {
	instance = new(Instance)
	db.NewSelect().
		Model(instance).
		Where("grab_id = ?", grabID)

	return instance
}

// Add a new instance
func insertInstance(db *bun.DB, instance *Instance) (err error) {
	ctx := context.Background()
	_, err = db.NewInsert().Model(instance).Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

// Update instance info if something changes
func updateInstance(db *bun.DB, grabID string, instance *Instance) (err error) {
	ctx := context.Background()
	_, err = db.NewUpdate().Model(instance).Where("grab_id = ?", grabID).Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func deleteInstance(db *bun.DB, teamID string) (err error) {
	ctx := context.Background()
	instance := new(Instance)
	_, err = db.NewDelete().Model(instance).Where("slack_team_id = ?", teamID).Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}
