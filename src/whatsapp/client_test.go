// SPDX-FileCopyrightText: 2025 Humaid Alqasimi
// SPDX-License-Identifier: Apache-2.0

package whatsapp

import (
	"testing"

	"go.mau.fi/whatsmeow/types"
)

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
