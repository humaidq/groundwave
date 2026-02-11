// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package whatsapp

import (
	"context"
	"errors"
	"testing"

	waStore "go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/types"
)

var (
	errTestLoaderShouldNotBeCalled = errors.New("loader should not be called")
	errTestLoaderBoom              = errors.New("boom")
)

func TestIsStaleDeviceStore(t *testing.T) {
	t.Parallel()

	jid := types.NewJID("11111111111", types.DefaultUserServer)

	tests := []struct {
		name      string
		device    *waStore.Device
		wantStale bool
	}{
		{
			name:      "nil device store is not stale",
			device:    nil,
			wantStale: false,
		},
		{
			name:      "new uninitialized store is not stale",
			device:    &waStore.Device{Initialized: false},
			wantStale: false,
		},
		{
			name:      "initialized store without jid is stale",
			device:    &waStore.Device{Initialized: true},
			wantStale: true,
		},
		{
			name:      "initialized store with jid is not stale",
			device:    &waStore.Device{ID: &jid, Initialized: true},
			wantStale: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isStaleDeviceStore(tt.device); got != tt.wantStale {
				t.Fatalf("isStaleDeviceStore() = %v, want %v", got, tt.wantStale)
			}
		})
	}
}

func TestRefreshStaleDeviceStore(t *testing.T) {
	t.Parallel()

	t.Run("reloads stale store", func(t *testing.T) {
		t.Parallel()

		staleStore := &waStore.Device{Initialized: true}
		freshStore := &waStore.Device{}
		called := false

		got, err := refreshStaleDeviceStore(context.Background(), staleStore, func(context.Context) (*waStore.Device, error) {
			called = true
			return freshStore, nil
		})
		if err != nil {
			t.Fatalf("refreshStaleDeviceStore() returned error: %v", err)
		}

		if !called {
			t.Fatal("expected loader to be called for stale store")
		}

		if got != freshStore {
			t.Fatal("expected refreshed store to be returned")
		}
	})

	t.Run("skips reload for non-stale store", func(t *testing.T) {
		t.Parallel()

		originalStore := &waStore.Device{Initialized: false}
		called := false

		got, err := refreshStaleDeviceStore(context.Background(), originalStore, func(context.Context) (*waStore.Device, error) {
			called = true
			return nil, errTestLoaderShouldNotBeCalled
		})
		if err != nil {
			t.Fatalf("refreshStaleDeviceStore() returned error: %v", err)
		}

		if called {
			t.Fatal("expected loader not to be called for non-stale store")
		}

		if got != originalStore {
			t.Fatal("expected original store to be returned for non-stale store")
		}
	})

	t.Run("wraps loader error", func(t *testing.T) {
		t.Parallel()

		loadErr := errTestLoaderBoom

		_, err := refreshStaleDeviceStore(context.Background(), &waStore.Device{Initialized: true}, func(context.Context) (*waStore.Device, error) {
			return nil, loadErr
		})
		if !errors.Is(err, loadErr) {
			t.Fatalf("expected wrapped loader error, got: %v", err)
		}
	})

	t.Run("fails when stale store has no loader", func(t *testing.T) {
		t.Parallel()

		_, err := refreshStaleDeviceStore(context.Background(), &waStore.Device{Initialized: true}, nil)
		if !errors.Is(err, errNoDeviceStoreLoader) {
			t.Fatalf("expected errNoDeviceStoreLoader, got: %v", err)
		}
	})
}

func TestResolveOtherPartyJID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		info types.MessageInfo
		want string
	}{
		{
			name: "outgoing device sent uses destination jid",
			info: types.MessageInfo{
				MessageSource: types.MessageSource{
					IsFromMe: true,
					Chat:     types.NewJID("11111111111", types.DefaultUserServer),
				},
				DeviceSentMeta: &types.DeviceSentMeta{DestinationJID: "22222222222@s.whatsapp.net"},
			},
			want: "22222222222@s.whatsapp.net",
		},
		{
			name: "outgoing lid destination prefers phone-addressed chat",
			info: types.MessageInfo{
				MessageSource: types.MessageSource{
					IsFromMe:     true,
					Chat:         types.NewJID("11111111111", types.DefaultUserServer),
					RecipientAlt: types.NewJID("99999999999", types.HiddenUserServer),
				},
				DeviceSentMeta: &types.DeviceSentMeta{DestinationJID: "88888888888@lid"},
			},
			want: "11111111111@s.whatsapp.net",
		},
		{
			name: "outgoing lid destination prefers phone recipient alt",
			info: types.MessageInfo{
				MessageSource: types.MessageSource{
					IsFromMe:     true,
					Chat:         types.NewJID("11111111111", types.HiddenUserServer),
					RecipientAlt: types.NewJID("33333333333", types.DefaultUserServer),
				},
				DeviceSentMeta: &types.DeviceSentMeta{DestinationJID: "88888888888@lid"},
			},
			want: "33333333333@s.whatsapp.net",
		},
		{
			name: "outgoing invalid destination falls back to phone-addressed chat",
			info: types.MessageInfo{
				MessageSource: types.MessageSource{
					IsFromMe:     true,
					Chat:         types.NewJID("11111111111", types.DefaultUserServer),
					RecipientAlt: types.NewJID("33333333333", types.HiddenUserServer),
				},
				DeviceSentMeta: &types.DeviceSentMeta{DestinationJID: "bad.dot.too.many@s.whatsapp.net"},
			},
			want: "11111111111@s.whatsapp.net",
		},
		{
			name: "incoming prefers sender",
			info: types.MessageInfo{
				MessageSource: types.MessageSource{
					Sender:    types.NewJID("44444444444", types.DefaultUserServer),
					SenderAlt: types.NewJID("55555555555", types.HiddenUserServer),
					Chat:      types.NewJID("66666666666", types.DefaultUserServer),
				},
			},
			want: "44444444444@s.whatsapp.net",
		},
		{
			name: "incoming lid sender prefers phone sender alt",
			info: types.MessageInfo{
				MessageSource: types.MessageSource{
					Sender:    types.NewJID("44444444444", types.HiddenUserServer),
					SenderAlt: types.NewJID("55555555555", types.DefaultUserServer),
					Chat:      types.NewJID("66666666666", types.HiddenUserServer),
				},
			},
			want: "55555555555@s.whatsapp.net",
		},
		{
			name: "incoming falls back to chat",
			info: types.MessageInfo{
				MessageSource: types.MessageSource{
					Chat: types.NewJID("77777777777", types.DefaultUserServer),
				},
			},
			want: "77777777777@s.whatsapp.net",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := resolveOtherPartyJID(tt.info).String(); got != tt.want {
				t.Fatalf("resolveOtherPartyJID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPreferPhoneNumberJID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		primary   types.JID
		alternate types.JID
		want      string
	}{
		{
			name:      "prefer hidden fallback to phone",
			primary:   types.NewJID("11111111111", types.HiddenUserServer),
			alternate: types.NewJID("22222222222", types.DefaultUserServer),
			want:      "22222222222@s.whatsapp.net",
		},
		{
			name:      "prefer hidden fallback to legacy phone",
			primary:   types.NewJID("11111111111", types.HiddenUserServer),
			alternate: types.NewJID("22222222222", types.LegacyUserServer),
			want:      "22222222222@c.us",
		},
		{
			name:      "prefer hosted lid fallback to phone",
			primary:   types.NewJID("11111111111", types.HostedLIDServer),
			alternate: types.NewJID("22222222222", types.DefaultUserServer),
			want:      "22222222222@s.whatsapp.net",
		},
		{
			name:      "keep primary when already phone",
			primary:   types.NewJID("11111111111", types.DefaultUserServer),
			alternate: types.NewJID("22222222222", types.HiddenUserServer),
			want:      "11111111111@s.whatsapp.net",
		},
		{
			name:      "empty primary uses alternate",
			alternate: types.NewJID("22222222222", types.DefaultUserServer),
			want:      "22222222222@s.whatsapp.net",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := preferPhoneNumberJID(tt.primary, tt.alternate).String(); got != tt.want {
				t.Fatalf("preferPhoneNumberJID() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsOutgoingMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		info types.MessageInfo
		want bool
	}{
		{
			name: "from me",
			info: types.MessageInfo{MessageSource: types.MessageSource{IsFromMe: true}},
			want: true,
		},
		{
			name: "device sent destination",
			info: types.MessageInfo{DeviceSentMeta: &types.DeviceSentMeta{DestinationJID: "22222222222@s.whatsapp.net"}},
			want: true,
		},
		{
			name: "device sent missing destination",
			info: types.MessageInfo{DeviceSentMeta: &types.DeviceSentMeta{}},
			want: false,
		},
		{
			name: "incoming normal message",
			info: types.MessageInfo{},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isOutgoingMessage(tt.info); got != tt.want {
				t.Fatalf("isOutgoingMessage() = %v, want %v", got, tt.want)
			}
		})
	}
}
