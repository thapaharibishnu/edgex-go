//
// Copyright (c) 2018 Cavium
//
// SPDX-License-Identifier: Apache-2.0
//

package logging

import (
	"fmt"
	"strconv"
	"time"

	"github.com/edgexfoundry/edgex-go/internal/support/logging/models"

	mgo "gopkg.in/mgo.v2"
	bson "gopkg.in/mgo.v2/bson"
)

type mongoLog struct {
	session *mgo.Session // Mongo database session
}

func connectToMongo() (*mgo.Session, error) {
	mongoDBDialInfo := &mgo.DialInfo{
		Addrs:    []string{Configuration.MongoURL + ":" + strconv.Itoa(Configuration.MongoPort)},
		Timeout:  time.Duration(Configuration.MongoConnectTimeout) * time.Millisecond,
		Database: Configuration.MongoDB,
		Username: Configuration.MongoUsername,
		Password: Configuration.MongoPassword,
	}

	ms, err := mgo.DialWithInfo(mongoDBDialInfo)
	if err != nil {
		return nil, err
	}

	ms.SetSocketTimeout(time.Duration(Configuration.SocketTimeout) * time.Millisecond)
	ms.SetMode(mgo.Monotonic, true)

	return ms, nil
}

func (ml *mongoLog) closeSession() {
	if ml.session != nil {
		ml.session.Close()
		ml.session = nil
	}
}

func (ml *mongoLog) add(le models.LogEntry) error {

	session := ml.session.Copy()
	defer session.Close()

	c := session.DB(Configuration.MongoDB).C(Configuration.MongoCollection)

	if err := c.Insert(le); err != nil {
		return err
	}

	return nil
}

func createConditions(conditions []bson.M, field string, elements []string) []bson.M {
	keyCond := []bson.M{}
	for _, value := range elements {
		keyCond = append(keyCond, bson.M{field: value})
	}

	return append(conditions, bson.M{"$or": keyCond})
}

func createQuery(criteria matchCriteria) bson.M {
	conditions := []bson.M{{}}

	if len(criteria.Labels) > 0 {
		conditions = createConditions(conditions, "labels", criteria.Labels)
	}

	if len(criteria.Keywords) > 0 {
		keyCond := []bson.M{}
		for _, key := range criteria.Keywords {
			regex := fmt.Sprintf(".*%s.*", key)
			keyCond = append(keyCond, bson.M{"message": bson.M{"$regex": regex}})
		}
		conditions = append(conditions, bson.M{"$or": keyCond})
	}

	if len(criteria.OriginServices) > 0 {
		conditions = createConditions(conditions, "originService", criteria.OriginServices)
	}

	if len(criteria.LogLevels) > 0 {
		conditions = createConditions(conditions, "logLevel", criteria.LogLevels)
	}

	if criteria.Start != 0 {
		conditions = append(conditions, bson.M{"created": bson.M{"$gt": criteria.Start}})
	}

	if criteria.End != 0 {
		conditions = append(conditions, bson.M{"created": bson.M{"$lt": criteria.End}})
	}

	return bson.M{"$and": conditions}

}

func (ml *mongoLog) remove(criteria matchCriteria) (int, error) {

	session := ml.session.Copy()
	defer session.Close()

	c := session.DB(Configuration.MongoDB).C(Configuration.MongoCollection)

	base := createQuery(criteria)

	info, err := c.RemoveAll(base)

	if err != nil {
		return 0, err
	}

	return info.Removed, nil
}

func (ml *mongoLog) find(criteria matchCriteria) ([]models.LogEntry, error) {
	session := ml.session.Copy()
	defer session.Close()

	c := session.DB(Configuration.MongoDB).C(Configuration.MongoCollection)

	le := []models.LogEntry{}

	base := createQuery(criteria)

	q := c.Find(base)

	if err := q.Limit(criteria.Limit).All(&le); err != nil {
		return le, err
	}

	return le, nil
}

func (ml *mongoLog) reset() {
	session := ml.session.Copy()
	defer session.Close()

	session.DB(Configuration.MongoDB).C(Configuration.MongoCollection).RemoveAll(bson.M{})
	return
}
