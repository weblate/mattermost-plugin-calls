package main

import (
	"encoding/json"
	"github.com/mattermost/mattermost-server/v6/model"
	"io/ioutil"
	"net/http"
)

type Agenda struct {
	Title string        `json:"title"`
	Items []*AgendaItem `json:"items"`
}

type AgendaItem struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	State          string `json:"state"`
	AssigneeID     string `json:"assignee_id"`
	Command        string `json:"command"`
	CommandLastRun int64  `json:"command_last_run"`
	DueDate        int64  `json:"due_date"`
}

func (p *Plugin) handleGetAgenda(w http.ResponseWriter, r *http.Request, token string, channelID string) {
	userID := r.Header.Get("Mattermost-User-Id")
	if !p.API.HasPermissionToChannel(userID, channelID, model.PermissionReadChannel) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	agenda := &Agenda{
		Title: "Up Next",
		Items: []*AgendaItem{},
	}

	blocks, err := p.fbStore.GetUpnextCards(userID, token, channelID)
	if err != nil {
		http.Error(w, "unable to get cards for agenda: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if len(blocks) > 0 {
		agenda.Items = make([]*AgendaItem, len(blocks))
		for i, block := range blocks {
			agenda.Items[i] = &AgendaItem{
				ID:    block.ID,
				Title: block.Title,
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(agenda); err != nil {
		p.LogError(err.Error())
	}
}

func (p *Plugin) handleUpdateAgendaItem(w http.ResponseWriter, r *http.Request, token string, channelID string) {
	userID := r.Header.Get("Mattermost-User-Id")
	if !p.API.HasPermissionToChannel(userID, channelID, model.PermissionReadChannel) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var item AgendaItem
	if err := json.Unmarshal(data, &item); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	status := ""
	switch item.State {
	case "closed":
		status = StatusDone
	default:
		status = StatusUpNext
	}

	err = p.fbStore.UpdateCardStatus(userID, token, item.ID, channelID, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var res httpResponse
	res.Code = http.StatusOK
	resBytes, err := json.Marshal(res)
	if err != nil {
		p.LogError(err.Error())
		http.Error(w, "", http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(resBytes); err != nil {
		p.LogError(err.Error())
	}
}

func (p *Plugin) handleAddAgendaItem(w http.ResponseWriter, r *http.Request, token string, channelID string) {
	userID := r.Header.Get("Mattermost-User-Id")
	if !p.API.HasPermissionToChannel(userID, channelID, model.PermissionReadChannel) {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var item AgendaItem
	if err := json.Unmarshal(data, &item); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// FIXME we were passing item.ID, but not anymore? figure why
	block, err := p.fbStore.AddCard(userID, token, channelID, item.Title)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	item.ID = block.ID

	resBytes, err := json.Marshal(item)
	if err != nil {
		p.LogError(err.Error())
		http.Error(w, "", http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	if _, err := w.Write(resBytes); err != nil {
		p.LogError(err.Error())
	}
}
