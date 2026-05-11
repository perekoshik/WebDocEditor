package documents

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"legaledit/internal/audit"
	"legaledit/internal/files"
	"legaledit/internal/onlyoffice"
)

type Handler struct {
	repo    *Repository
	store   *files.Storage
	builder *onlyoffice.ConfigBuilder
	convert *onlyoffice.Client
	http    *http.Client
	audit   *audit.Logger
	log     *slog.Logger
}

func NewHandler(repo *Repository, store *files.Storage, builder *onlyoffice.ConfigBuilder, convert *onlyoffice.Client, httpClient *http.Client, auditLog *audit.Logger, log *slog.Logger) *Handler {
	return &Handler{repo: repo, store: store, builder: builder, convert: convert, http: httpClient, audit: auditLog, log: log}
}

const uploadLimit = 64 << 20

func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(uploadLimit); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file field required")
		return
	}
	defer file.Close()

	ext := files.Ext(header.Filename)
	if ext == "" {
		writeError(w, http.StatusBadRequest, "file extension required")
		return
	}

	stored, err := h.store.Save(file, ext)
	if err != nil {
		h.log.Error("save file", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	title := files.StripExt(header.Filename)
	if title == "" {
		title = header.Filename
	}

	isTemplate := r.FormValue("is_template") == "true"

	doc, err := h.repo.Create(r.Context(), title, header.Filename, stored, isTemplate)
	if err != nil {
		h.log.Error("create document", "err", err)
		_ = h.store.Delete(stored)
		writeError(w, http.StatusInternalServerError, "failed to create document")
		return
	}
	action := "document.upload"
	if isTemplate {
		action = "template.upload"
	}
	h.audit.Record(r.Context(), action, &doc.ID, map[string]any{
		"title":    doc.Title,
		"filename": doc.Filename,
		"size":     header.Size,
	})
	writeJSON(w, http.StatusCreated, doc)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	isTemplate := r.URL.Query().Get("template") == "true"
	docs, err := h.repo.List(r.Context(), q, isTemplate)
	if err != nil {
		h.log.Error("list", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to list documents")
		return
	}
	writeJSON(w, http.StatusOK, docs)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	doc, err := h.repo.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		h.log.Error("get", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to load document")
		return
	}

	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	userName := strings.TrimSpace(r.URL.Query().Get("user_name"))
	if userID == "" {
		userID = uuid.NewString()
	}
	if userName == "" {
		userName = "Аноним"
	}

	cfg := h.builder.Build(doc.ID.String(), doc.Version, doc.Filename, doc.Title, onlyoffice.UserInfo{ID: userID, Name: userName})

	writeJSON(w, http.StatusOK, map[string]any{
		"document":     doc,
		"editorConfig": cfg,
	})
}

func (h *Handler) File(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	doc, err := h.repo.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load document")
		return
	}
	f, err := h.store.Open(doc.StoragePath)
	if err != nil {
		h.log.Error("open file", "err", err, "path", doc.StoragePath)
		writeError(w, http.StatusInternalServerError, "failed to open file")
		return
	}
	defer f.Close()

	ext := files.Ext(doc.Filename)
	ct := mime.TypeByExtension(ext)
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", doc.Filename))
	_, _ = io.Copy(w, f)
}

func (h *Handler) Instantiate(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	tpl, err := h.repo.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "template not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load template")
		return
	}
	if !tpl.IsTemplate {
		writeError(w, http.StatusBadRequest, "document is not a template")
		return
	}

	var body struct {
		Title string `json:"title"`
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	title := strings.TrimSpace(body.Title)
	if title == "" {
		title = tpl.Title
	}

	src, err := h.store.Open(tpl.StoragePath)
	if err != nil {
		h.log.Error("open template file", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to read template")
		return
	}
	defer src.Close()

	stored, err := h.store.Save(src, files.Ext(tpl.Filename))
	if err != nil {
		h.log.Error("copy template file", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to copy template")
		return
	}

	doc, err := h.repo.Create(r.Context(), title, tpl.Filename, stored, false)
	if err != nil {
		_ = h.store.Delete(stored)
		h.log.Error("create from template", "err", err)
		writeError(w, http.StatusInternalServerError, "failed to create document")
		return
	}
	h.audit.Record(r.Context(), "template.instantiate", &doc.ID, map[string]any{
		"template_id": tpl.ID,
		"title":       doc.Title,
	})
	writeJSON(w, http.StatusCreated, doc)
}

func (h *Handler) VersionFile(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	versionStr := chi.URLParam(r, "version")
	var version int
	if _, err := fmt.Sscanf(versionStr, "%d", &version); err != nil || version < 1 {
		writeError(w, http.StatusBadRequest, "invalid version")
		return
	}
	doc, err := h.repo.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load document")
		return
	}
	path, err := h.repo.GetVersionPath(r.Context(), id, version)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "version not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load version")
		return
	}
	f, err := h.store.Open(path)
	if err != nil {
		h.log.Error("open version file", "err", err, "path", path)
		writeError(w, http.StatusInternalServerError, "failed to open file")
		return
	}
	defer f.Close()

	ct := mime.TypeByExtension(files.Ext(doc.Filename))
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%q", doc.Filename))
	_, _ = io.Copy(w, f)
}

type callbackPayload struct {
	Key    string `json:"key"`
	Status int    `json:"status"`
	URL    string `json:"url"`
}

func (h *Handler) Callback(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeOO(w, 1)
		return
	}
	var p callbackPayload
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		h.log.Error("callback decode", "err", err)
		writeOO(w, 1)
		return
	}
	h.log.Info("callback", "id", id, "status", p.Status, "key", p.Key)

	if p.Status == 2 || p.Status == 6 {
		newVersion, err := h.persistVersion(r.Context(), id, p.URL)
		if err != nil {
			h.log.Error("persist version", "err", err)
			writeOO(w, 1)
			return
		}
		h.audit.Record(r.Context(), "document.save", &id, map[string]any{
			"version": newVersion,
			"status":  p.Status,
		})
	}
	writeOO(w, 0)
}

func (h *Handler) persistVersion(ctx context.Context, id uuid.UUID, url string) (int, error) {
	if url == "" {
		return 0, errors.New("empty url")
	}
	doc, err := h.repo.Get(ctx, id)
	if err != nil {
		return 0, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	resp, err := h.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("download status %d", resp.StatusCode)
	}

	ext := files.Ext(doc.Filename)
	stored, err := h.store.Save(resp.Body, ext)
	if err != nil {
		return 0, err
	}
	updated, err := h.repo.AddVersion(ctx, id, stored)
	if err != nil {
		_ = h.store.Delete(stored)
		return 0, err
	}
	return updated.Version, nil
}

func (h *Handler) Versions(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	vs, err := h.repo.Versions(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load versions")
		return
	}
	writeJSON(w, http.StatusOK, vs)
}

func (h *Handler) Rename(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var body struct {
		Title string `json:"title"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid body")
		return
	}
	body.Title = strings.TrimSpace(body.Title)
	if body.Title == "" {
		writeError(w, http.StatusBadRequest, "title required")
		return
	}
	doc, err := h.repo.UpdateTitle(r.Context(), id, body.Title)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to rename")
		return
	}
	h.audit.Record(r.Context(), "document.rename", &doc.ID, map[string]any{"title": doc.Title})
	writeJSON(w, http.StatusOK, doc)
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	paths, err := h.repo.Delete(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete")
		return
	}
	for _, p := range paths {
		if err := h.store.Delete(p); err != nil {
			h.log.Warn("delete file", "err", err, "path", p)
		}
	}
	h.audit.Record(r.Context(), "document.delete", &id, map[string]any{"file_count": len(paths)})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) Export(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format != "pdf" && format != "docx" {
		writeError(w, http.StatusBadRequest, "format must be pdf or docx")
		return
	}
	doc, err := h.repo.Get(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "not found")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load document")
		return
	}

	sourceExt := strings.TrimPrefix(files.Ext(doc.Filename), ".")
	if sourceExt == "" {
		sourceExt = "docx"
	}
	sourceURL := fmt.Sprintf("%s/api/documents/%s/file", h.builder.InternalAPIURL, doc.ID)
	key := fmt.Sprintf("%s_%d_%s_%s", doc.ID, doc.Version, format, uuid.NewString()[:8])
	title := files.StripExt(doc.Filename) + "." + format

	fileURL, err := h.convert.Convert(r.Context(), key, sourceExt, format, title, sourceURL)
	if err != nil {
		h.log.Error("convert", "err", err)
		writeError(w, http.StatusBadGateway, "convert failed")
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, fileURL, nil)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "request build failed")
		return
	}
	resp, err := h.http.Do(req)
	if err != nil {
		writeError(w, http.StatusBadGateway, "download failed")
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		writeError(w, http.StatusBadGateway, "download status not ok")
		return
	}

	ct := mime.TypeByExtension("." + format)
	if ct == "" {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filepath.Base(title)))
	h.audit.Record(r.Context(), "document.export", &doc.ID, map[string]any{"format": format})
	_, _ = io.Copy(w, resp.Body)
}

func parseID(r *http.Request) (uuid.UUID, error) {
	return uuid.Parse(chi.URLParam(r, "id"))
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func writeOO(w http.ResponseWriter, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"error":%d}`, code)))
}
