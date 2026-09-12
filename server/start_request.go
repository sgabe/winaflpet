package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/rs/xid"
)

type agentHTTPError struct {
	Code    int
	Message string
}

func (e *agentHTTPError) Error() string { return fmt.Sprintf("Agent HTTP %d: %s", e.Code, e.Message) }
func (j *Job) pendingStart(fid int) (string, Job, error) {
	var id, raw string
	err := db.QueryRow("SELECT request_id,payload FROM job_start_requests WHERE guid=? AND fid=?", j.GUID.String(), fid).Scan(&id, &raw)
	if err == nil {
		var saved Job
		err = json.Unmarshal([]byte(raw), &saved)
		return id, saved, err
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return "", Job{}, err
	}
	id = xid.New().String()
	b, err := json.Marshal(j)
	if err != nil {
		return "", Job{}, err
	}
	_, err = db.Exec("INSERT INTO job_start_requests(guid,fid,request_id,payload) VALUES(?,?,?,?)", j.GUID.String(), fid, id, string(b))
	return id, *j, err
}
func (j *Job) clearPendingStart(fid int) error {
	_, err := db.Exec("DELETE FROM job_start_requests WHERE guid=? AND fid=?", j.GUID.String(), fid)
	return err
}

func (j *Job) pendingStartIDs() ([]string, error) {
	rows, err := db.Query("SELECT request_id FROM job_start_requests WHERE guid=?", j.GUID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
