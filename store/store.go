// init mongodb session and provide common functions

package store

import (
	"strings"
	"time"

	conf "github.com/irisnet/irishub-sync/conf/db"
	"github.com/irisnet/irishub-sync/module/logger"

	"errors"
	"fmt"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

var (
	session *mgo.Session
	docs    []Docs
)

func RegisterDocs(d Docs) {
	docs = append(docs, d)
}

func Start() {
	addrs := strings.Split(conf.Addrs, ",")
	dialInfo := &mgo.DialInfo{
		Addrs:     addrs,
		Database:  conf.Database,
		Username:  conf.User,
		Password:  conf.Passwd,
		Direct:    false,
		Timeout:   time.Second * 10,
		PoolLimit: 4096, // Session.SetPoolLimit
	}

	var err error
	session, err = mgo.DialWithInfo(dialInfo)
	session.SetMode(mgo.Monotonic, true)
	if err != nil {
		logger.Error(err.Error())
	}
}

func Stop() {
	logger.Info("release resource :mongoDb")
	session.Close()
}

func getSession() *mgo.Session {
	// max session num is 4096
	return session.Clone()
}

// get collection object
func ExecCollection(collection string, s func(*mgo.Collection) error) error {
	session := getSession()
	defer session.Close()
	c := session.DB(conf.Database).C(collection)
	return s(c)
}

func Find(collection string, query interface{}) *mgo.Query {
	session := getSession()
	defer session.Close()
	c := session.DB(conf.Database).C(collection)
	return c.Find(query)
}

func Save(h Docs) error {
	save := func(c *mgo.Collection) error {
		pk := h.PkKvPair()
		n, _ := c.Find(pk).Count()
		if n >= 1 {
			errMsg := fmt.Sprintf("Record exists")
			return errors.New(errMsg)
		}
		logger.Info("Save document", logger.String("table", h.Name()), logger.Any("content", h))
		return c.Insert(h)
	}
	return ExecCollection(h.Name(), save)
}

func SaveAll(collectionName string, docs []interface{}) error {
	session := getSession()
	defer session.Close()

	c := session.DB(conf.Database).C(collectionName)
	return c.Insert(docs...)
}

func SaveOrUpdate(h Docs) error {
	save := func(c *mgo.Collection) error {
		n, err := c.Find(h.PkKvPair()).Count()
		if err != nil {
			logger.Error("Store Find error", logger.String("err", err.Error()))
		}

		if n >= 1 {
			return Update(h)
		}
		logger.Info("Save document", logger.String("table", h.Name()), logger.Any("content", h))
		return c.Insert(h)
	}

	return ExecCollection(h.Name(), save)
}

func Update(h Docs) error {
	update := func(c *mgo.Collection) error {
		key := h.PkKvPair()
		logger.Info("update document", logger.String("table", h.Name()), logger.Any("conditions", h.PkKvPair()))
		return c.Update(key, h)
	}
	return ExecCollection(h.Name(), update)
}

func Delete(h Docs) error {
	remove := func(c *mgo.Collection) error {
		key := h.PkKvPair()
		logger.Info("delete document", logger.String("table", h.Name()), logger.Any("conditions", h.PkKvPair()))
		return c.Remove(key)
	}
	return ExecCollection(h.Name(), remove)
}

func Query(collectionName string, query bson.M, sort string, fields bson.M, skip int, limit int) (results []interface{}, err error) {
	callback := func(c *mgo.Collection) error {
		logger.Info("query document", logger.String("table", collectionName), logger.Any("conditions", query))
		return c.Find(query).Sort(sort).Select(fields).Skip(skip).Limit(limit).All(&results)
	}
	return results, ExecCollection(collectionName, callback)
}
