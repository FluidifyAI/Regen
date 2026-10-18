package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/FluidifyAI/Regen/backend/internal/models"
	"github.com/FluidifyAI/Regen/backend/internal/repository"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeHolidayRepo implements only the holiday methods; the embedded nil interface
// panics if a handler touches anything else, which keeps these tests honest.
type fakeHolidayRepo struct {
	repository.ScheduleRepository
	scheduleID  uuid.UUID
	holidays    map[uuid.UUID]*models.ScheduleHoliday
	lastInclude bool
}

func newFakeHolidayRepo(scheduleID uuid.UUID) *fakeHolidayRepo {
	return &fakeHolidayRepo{scheduleID: scheduleID, holidays: map[uuid.UUID]*models.ScheduleHoliday{}}
}

func (f *fakeHolidayRepo) GetByID(id uuid.UUID) (*models.Schedule, error) {
	if id != f.scheduleID {
		return nil, &repository.NotFoundError{Resource: "schedule", ID: id}
	}
	return &models.Schedule{ID: id}, nil
}

func (f *fakeHolidayRepo) seed(country, date, name, source string, hidden bool) *models.ScheduleHoliday {
	h := &models.ScheduleHoliday{ID: uuid.New(), ScheduleID: f.scheduleID, CountryCode: country,
		Date: models.DateOnly(date), Name: name, Source: source, Hidden: hidden}
	f.holidays[h.ID] = h
	return h
}

func (f *fakeHolidayRepo) ListHolidays(_ uuid.UUID, _, _ time.Time, includeHidden bool) ([]models.ScheduleHoliday, error) {
	f.lastInclude = includeHidden
	var out []models.ScheduleHoliday
	for _, h := range f.holidays {
		if h.Hidden && !includeHidden {
			continue
		}
		out = append(out, *h)
	}
	return out, nil
}

func (f *fakeHolidayRepo) GetHoliday(scheduleID, id uuid.UUID) (*models.ScheduleHoliday, error) {
	h, ok := f.holidays[id]
	if !ok || h.ScheduleID != scheduleID {
		return nil, &repository.NotFoundError{Resource: "holiday", ID: id}
	}
	cp := *h
	return &cp, nil
}

func (f *fakeHolidayRepo) CreateHoliday(h *models.ScheduleHoliday) error {
	for _, e := range f.holidays {
		if e.ScheduleID == h.ScheduleID && e.CountryCode == h.CountryCode && e.Date == h.Date {
			return &repository.AlreadyExistsError{Resource: "holiday", Field: "date", Value: h.Date}
		}
	}
	h.ID = uuid.New()
	cp := *h
	f.holidays[h.ID] = &cp
	return nil
}

func (f *fakeHolidayRepo) UpdateHoliday(h *models.ScheduleHoliday) error {
	if _, ok := f.holidays[h.ID]; !ok {
		return &repository.NotFoundError{Resource: "holiday", ID: h.ID}
	}
	cp := *h
	f.holidays[h.ID] = &cp
	return nil
}

func (f *fakeHolidayRepo) DeleteHoliday(scheduleID, id uuid.UUID) error {
	h, ok := f.holidays[id]
	if !ok || h.ScheduleID != scheduleID {
		return &repository.NotFoundError{Resource: "holiday", ID: id}
	}
	delete(f.holidays, id)
	return nil
}

func holidayRouter(repo *fakeHolidayRepo, role string) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	// "local_user" is the context key the auth middleware stores the user under.
	r.Use(func(c *gin.Context) {
		if role != "" {
			c.Set("local_user", &models.User{Role: models.UserRole(role)})
		}
		c.Next()
	})
	r.GET("/schedules/:id/holidays", GetHolidays(repo))
	r.POST("/schedules/:id/holidays", CreateHoliday(repo))
	r.PATCH("/schedules/:id/holidays/:holiday_id", UpdateHoliday(repo))
	r.DELETE("/schedules/:id/holidays/:holiday_id", DeleteHoliday(repo))
	return r
}

func doHoliday(r *gin.Engine, method, path string, body interface{}) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestGetHolidays_HiddenOnlyForAdmins(t *testing.T) {
	sid := uuid.New()
	repo := newFakeHolidayRepo(sid)
	repo.seed("AU", "2026-10-05", "NSW only", models.HolidaySourceFeed, true)
	path := "/schedules/" + sid.String() + "/holidays?include_hidden=true"

	doHoliday(holidayRouter(repo, "admin"), http.MethodGet, path, nil)
	assert.True(t, repo.lastInclude, "admin should see hidden holidays")

	doHoliday(holidayRouter(repo, "member"), http.MethodGet, path, nil)
	assert.False(t, repo.lastInclude, "include_hidden must be ignored for non-admins")

	doHoliday(holidayRouter(repo, ""), http.MethodGet, path, nil)
	assert.False(t, repo.lastInclude, "include_hidden must be ignored without a user")
}

func TestCreateHoliday(t *testing.T) {
	sid := uuid.New()
	repo := newFakeHolidayRepo(sid)
	r := holidayRouter(repo, "admin")
	path := "/schedules/" + sid.String() + "/holidays"

	w := doHoliday(r, http.MethodPost, path, map[string]string{"date": "2026-11-03", "name": "  Company day  "})
	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())
	var got map[string]interface{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "Company day", got["name"])
	assert.Equal(t, "manual", got["source"])
	assert.Equal(t, "custom", got["country_code"])

	assert.Equal(t, http.StatusConflict,
		doHoliday(r, http.MethodPost, path, map[string]string{"date": "2026-11-03", "name": "Again"}).Code)
	assert.Equal(t, http.StatusBadRequest,
		doHoliday(r, http.MethodPost, path, map[string]string{"date": "03/11/2026", "name": "Bad date"}).Code)
	assert.Equal(t, http.StatusBadRequest,
		doHoliday(r, http.MethodPost, path, map[string]string{"date": "2026-11-04", "name": "   "}).Code)
	long := make([]byte, 256)
	for i := range long {
		long[i] = 'x'
	}
	assert.Equal(t, http.StatusBadRequest,
		doHoliday(r, http.MethodPost, path, map[string]string{"date": "2026-11-04", "name": string(long)}).Code)
	assert.Equal(t, http.StatusNotFound,
		doHoliday(r, http.MethodPost, "/schedules/"+uuid.New().String()+"/holidays",
			map[string]string{"date": "2026-11-05", "name": "x"}).Code)
}

func TestUpdateHoliday_RenameMarksManual_HideAndRestore(t *testing.T) {
	sid := uuid.New()
	repo := newFakeHolidayRepo(sid)
	h := repo.seed("AU", "2026-10-05", "Labour Day", models.HolidaySourceFeed, false)
	r := holidayRouter(repo, "admin")
	path := "/schedules/" + sid.String() + "/holidays/" + h.ID.String()

	w := doHoliday(r, http.MethodPatch, path, map[string]interface{}{"name": "King's Birthday"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	assert.Equal(t, "King's Birthday", repo.holidays[h.ID].Name)
	assert.Equal(t, models.HolidaySourceManual, repo.holidays[h.ID].Source, "a rename must stick across syncs")

	require.Equal(t, http.StatusOK, doHoliday(r, http.MethodPatch, path, map[string]interface{}{"hidden": true}).Code)
	assert.True(t, repo.holidays[h.ID].Hidden)
	require.Equal(t, http.StatusOK, doHoliday(r, http.MethodPatch, path, map[string]interface{}{"hidden": false}).Code)
	assert.False(t, repo.holidays[h.ID].Hidden)

	assert.Equal(t, http.StatusBadRequest, doHoliday(r, http.MethodPatch, path, map[string]interface{}{}).Code,
		"an empty patch is a client error")
	assert.Equal(t, http.StatusBadRequest, doHoliday(r, http.MethodPatch, path, map[string]interface{}{"name": " "}).Code)
}

func TestHolidayHandlers_ScopedToSchedule(t *testing.T) {
	sid, other := uuid.New(), uuid.New()
	repo := newFakeHolidayRepo(sid)
	h := repo.seed("AU", "2026-10-05", "Labour Day", models.HolidaySourceFeed, false)
	r := holidayRouter(repo, "admin")

	// A holiday of one schedule must not be reachable through another schedule's URL.
	path := "/schedules/" + other.String() + "/holidays/" + h.ID.String()
	assert.Equal(t, http.StatusNotFound, doHoliday(r, http.MethodPatch, path, map[string]interface{}{"hidden": true}).Code)
	assert.Equal(t, http.StatusNotFound, doHoliday(r, http.MethodDelete, path, nil).Code)
	assert.False(t, repo.holidays[h.ID].Hidden)
}

func TestDeleteHoliday_FeedHidesCustomDeletes(t *testing.T) {
	sid := uuid.New()
	repo := newFakeHolidayRepo(sid)
	feed := repo.seed("AU", "2026-10-05", "NSW only", models.HolidaySourceFeed, false)
	renamed := repo.seed("AU", "2026-10-06", "Renamed feed day", models.HolidaySourceManual, false)
	custom := repo.seed(models.HolidayCountryCustom, "2026-11-03", "Company day", models.HolidaySourceManual, false)
	r := holidayRouter(repo, "admin")
	base := "/schedules/" + sid.String() + "/holidays/"

	require.Equal(t, http.StatusNoContent, doHoliday(r, http.MethodDelete, base+feed.ID.String(), nil).Code)
	require.Contains(t, repo.holidays, feed.ID, "a feed row must be kept so the sync cannot re-add it")
	assert.True(t, repo.holidays[feed.ID].Hidden)

	// A renamed feed row is source=manual but still has a real country code, so a hard
	// delete would let the next sync re-import the original. It must be hidden instead.
	require.Equal(t, http.StatusNoContent, doHoliday(r, http.MethodDelete, base+renamed.ID.String(), nil).Code)
	require.Contains(t, repo.holidays, renamed.ID)
	assert.True(t, repo.holidays[renamed.ID].Hidden)

	require.Equal(t, http.StatusNoContent, doHoliday(r, http.MethodDelete, base+custom.ID.String(), nil).Code)
	assert.NotContains(t, repo.holidays, custom.ID, "an admin-added holiday is deleted outright")

	assert.Equal(t, http.StatusNotFound, doHoliday(r, http.MethodDelete, base+uuid.New().String(), nil).Code)
}
