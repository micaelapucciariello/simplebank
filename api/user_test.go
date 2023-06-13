package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	mockdb "github.com/micaelapucciariello/simplebank/db/mock"
	db "github.com/micaelapucciariello/simplebank/db/sqlc"
	"github.com/micaelapucciariello/simplebank/utils"
)

type eqCreateUserParamsMatcher struct {
	arg      db.CreateUserParams
	password string
}

func (e eqCreateUserParamsMatcher) Matches(x interface{}) bool {
	arg, ok := x.(db.CreateUserParams)
	if !ok {
		return false
	}
	e.arg.HashedPassword = arg.HashedPassword
	return reflect.DeepEqual(e.arg, arg)
}

func EqCreateUserParams(arg db.CreateUserParams, password string) gomock.Matcher {
	return &eqCreateUserParamsMatcher{
		arg:      arg,
		password: password,
	}
}

func (e eqCreateUserParamsMatcher) String() string {
	return fmt.Sprintf("matches arg %v and password %v", e.arg, e.password)
}

func TestGetUserAPI(t *testing.T) {
	user, _ := randomUser()

	testCases := []struct {
		name          string
		username      string
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:     "happy path get user",
			username: user.Username,
			buildStubs: func(store *mockdb.MockStore) {
				// build stubs
				store.EXPECT().GetUser(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(user, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				// check response
				require.Equal(t, http.StatusOK, recorder.Code)
				validateResponseUser(t, recorder.Body, user)
			},
		},
		{
			name:     "user not found",
			username: user.Username,
			buildStubs: func(store *mockdb.MockStore) {
				// build stubs
				store.EXPECT().GetUser(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.User{}, sql.ErrNoRows)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				// check response
				require.Equal(t, http.StatusNotFound, recorder.Code)
			},
		},
		{
			name:     "internal server error",
			username: user.Username,
			buildStubs: func(store *mockdb.MockStore) {
				// build stubs
				store.EXPECT().GetUser(gomock.Any(), gomock.Eq(user.Username)).
					Times(1).
					Return(db.User{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				// check response
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
		{
			name:     "invalid request",
			username: "",
			buildStubs: func(store *mockdb.MockStore) {
				// build stubs
				store.EXPECT().GetUser(gomock.Any(), gomock.Any()).
					Times(1)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				// check response
				require.Equal(t, http.StatusBadRequest, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			recorder := httptest.NewRecorder()
			server := NewServer(store)

			url := fmt.Sprintf("/users/%v", tc.username)

			request, err := http.NewRequest(http.MethodGet, url, nil)
			// check request
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func TestCreateUserAPI(t *testing.T) {
	user, password := randomUser()

	testCases := []struct {
		name          string
		user          db.User
		buildStubs    func(store *mockdb.MockStore)
		checkResponse func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name: "happy path create user",
			user: user,
			buildStubs: func(store *mockdb.MockStore) {
				// build stubs
				store.EXPECT().CreateUser(gomock.Any(), EqCreateUserParams(db.CreateUserParams{
					Username:       user.Username,
					FullName:       user.FullName,
					Email:          user.Email,
					HashedPassword: user.HashedPassword,
				}, password)).
					Times(1).
					Return(user, nil)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				// check response
				require.Equal(t, http.StatusOK, recorder.Code)
				validateResponseUser(t, recorder.Body, user)
			},
		},
		{
			name: "internal server error",
			user: user,
			buildStubs: func(store *mockdb.MockStore) {
				// build stubs
				store.EXPECT().CreateUser(gomock.Any(), EqCreateUserParams(db.CreateUserParams{
					Username: user.Username,
					FullName: user.FullName,
					Email:    user.Email,
				}, password)).
					Times(1).
					Return(db.User{}, sql.ErrConnDone)
			},
			checkResponse: func(t *testing.T, recorder *httptest.ResponseRecorder) {
				// check response
				require.Equal(t, http.StatusInternalServerError, recorder.Code)
			},
		},
	}

	for i := range testCases {
		tc := testCases[i]

		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()
			store := mockdb.NewMockStore(ctrl)
			tc.buildStubs(store)

			recorder := httptest.NewRecorder()
			server := NewServer(store)

			url := fmt.Sprintf("/users")

			body := fmt.Sprintf(`{"username": "%v", "full_name": "%v", "email": "%v", "password": "%v"}`, user.Username, user.FullName, user.Email, password)
			jsonBody := []byte(body)
			bodyReader := bytes.NewReader(jsonBody)

			request, err := http.NewRequest(http.MethodPost, url, bodyReader)
			// check request
			require.NoError(t, err)

			server.router.ServeHTTP(recorder, request)
			tc.checkResponse(t, recorder)
		})
	}
}

func randomUser() (db.User, string) {
	password := utils.RandomString(10)
	hashedPassword, _ := utils.HashPassword(password)
	user := db.User{
		Username:       utils.RandomOwner(),
		HashedPassword: hashedPassword,
		FullName:       utils.RandomOwner(),
		Email:          utils.RandomEmail(),
	}

	return user, password
}

func validateResponseUser(t *testing.T, body *bytes.Buffer, acc db.User) {
	data, err := ioutil.ReadAll(body)
	require.NoError(t, err)

	var rspUser db.User
	err = json.Unmarshal(data, &rspUser)
	require.NoError(t, err)
	require.Equal(t, acc, rspUser)
}
