package httpapi

import (
	"net/http"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

var launcherAppKeyPattern = regexp.MustCompile(`^[a-f0-9]{32,128}$`)

// handleCoreLauncherCatalog receives the local app inventory from an authenticated
// Agent and returns centrally managed categories. The Agent never decides a manager's
// override; it only supplies the apps installed on this particular PC.
func (s *Server) handleCoreLauncherCatalog(w http.ResponseWriter, r *http.Request) {
	if !s.requireCore(w, r) {
		return
	}
	var req struct {
		ExternalPCID string `json:"external_pc_id"`
		Apps         []struct {
			Key      string `json:"key"`
			Name     string `json:"name"`
			ExePath  string `json:"exe_path"`
			Args     string `json:"args"`
			Category string `json:"category"`
		} `json:"apps"`
	}
	if !mobileDecode(w, r, &req) {
		return
	}
	req.ExternalPCID = strings.TrimSpace(req.ExternalPCID)
	if req.ExternalPCID == "" || len(req.Apps) > 500 {
		writeError(w, http.StatusBadRequest, "invalid_launcher_catalog")
		return
	}
	var clubID, pcID string
	err := s.db.QueryRow(r.Context(), `SELECT club_id::text,id::text FROM pc_refs WHERE external_pc_id=$1 AND status_cache <> 'deleted' ORDER BY created_at DESC LIMIT 1`, req.ExternalPCID).Scan(&clubID, &pcID)
	if err != nil {
		writeError(w, http.StatusNotFound, "pc not found")
		return
	}
	tx, err := s.db.Begin(r.Context())
	if err != nil {
		mobileInternal(w)
		return
	}
	defer tx.Rollback(r.Context())
	categories := make(map[string]string, len(req.Apps))
	for _, app := range req.Apps {
		key, name := strings.ToLower(strings.TrimSpace(app.Key)), strings.TrimSpace(app.Name)
		if !launcherAppKeyPattern.MatchString(key) || name == "" || len(name) > 160 || len(app.ExePath) > 1024 || len(app.Args) > 1024 {
			writeError(w, http.StatusBadRequest, "invalid_launcher_app")
			return
		}
		defaultCategory := normalizeLauncherCategory(app.Category)
		var category string
		err = tx.QueryRow(r.Context(), `INSERT INTO launcher_apps(club_id,pc_ref_id,app_key,name,exe_path,args,category,last_seen_at,updated_at)
            VALUES($1,$2,$3,$4,$5,$6,$7,now(),now())
            ON CONFLICT(pc_ref_id,app_key) DO UPDATE SET name=EXCLUDED.name,exe_path=EXCLUDED.exe_path,args=EXCLUDED.args,last_seen_at=now(),updated_at=now()
            RETURNING category`, clubID, pcID, key, name, strings.TrimSpace(app.ExePath), strings.TrimSpace(app.Args), defaultCategory).Scan(&category)
		if err != nil {
			mobileInternal(w)
			return
		}
		categories[key] = category
	}
	if err = tx.Commit(r.Context()); err != nil {
		mobileInternal(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"categories": categories})
}

func (s *Server) handleBackofficeLauncherApps(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	clubID := r.PathValue("club_id")
	if _, ok := s.requireClubRole(w, r, auth, clubID, "owner", "manager"); !ok {
		return
	}
	rows, err := s.queryMaps(r.Context(), `SELECT a.id::text,a.app_key,a.name,a.exe_path,a.args,a.category,a.source,a.last_seen_at,
        p.id::text AS pc_id,p.label AS pc_label,p.external_pc_id
      FROM launcher_apps a JOIN pc_refs p ON p.id=a.pc_ref_id
      WHERE a.club_id=$1 ORDER BY p.number,a.name`, clubID)
	if err != nil {
		mobileInternal(w)
		return
	}
	if rows == nil {
		rows = []map[string]any{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"apps": rows})
}

func (s *Server) handleBackofficeLauncherAppCategory(w http.ResponseWriter, r *http.Request) {
	auth, ok := s.requireAuth(w, r)
	if !ok {
		return
	}
	appID := r.PathValue("app_id")
	var clubID string
	if err := s.db.QueryRow(r.Context(), `SELECT club_id::text FROM launcher_apps WHERE id=$1`, appID).Scan(&clubID); err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "launcher app not found")
			return
		}
		mobileInternal(w)
		return
	}
	if _, ok := s.requireClubRole(w, r, auth, clubID, "owner", "manager"); !ok {
		return
	}
	var req struct {
		Category string `json:"category"`
	}
	if !mobileDecode(w, r, &req) {
		return
	}
	category := normalizeLauncherCategory(req.Category)
	if category != strings.ToLower(strings.TrimSpace(req.Category)) {
		writeError(w, http.StatusBadRequest, "invalid_launcher_category")
		return
	}
	_, err := s.db.Exec(r.Context(), `UPDATE launcher_apps SET category=$2,updated_at=now() WHERE id=$1`, appID, category)
	if err != nil {
		mobileInternal(w)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "category": category})
}

func normalizeLauncherCategory(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "shooter":
		return "shooter"
	case "strategy":
		return "strategy"
	default:
		return "other"
	}
}
