package tools

import (
	"database/sql"
	_ "github.com/mattn/go-sqlite3"
	"strconv"
	"time"
)

func Connect() (db *sql.DB, err error) {
	db, err = sql.Open("sqlite3", "db/monitor.db")
	return db, err
}

type Ip struct {
	ip       int
	location string
	wTime    int64
}

//从数据库中查询ip
func FindIp(ip int, db *sql.DB) (ipInfo Ip, err error) {
	rows, err := db.Query("SELECT * FROM ip where ip =" + strconv.Itoa(ip))
	if err == nil {
		var id int
		for rows.Next() {
			err = rows.Scan(&id, &ipInfo.ip, &ipInfo.location, &ipInfo.wTime)
		}

	}
	return ipInfo, err
}

func Insert(db *sql.DB, ipInt int, location string) (err error) {
	//插入记录
	stmt, err := db.Prepare("INSERT INTO ip(ip, location, w_time) values(?,?,?)")
	if err != nil {
		return err
	}
	_, err = stmt.Exec(ipInt, location, time.Now().Unix())
	return err
}

func Update(db *sql.DB, ipInt int) {
	stmt, err := db.Prepare("update ip set w_time=? where ip=?")
	checkErr(err)
	_, err = stmt.Exec(time.Now().Unix(), ipInt)
	checkErr(err)
	//affect, err := res.RowsAffected()
	//checkErr(err)
	//return err
}
