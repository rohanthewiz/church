# Session: Android Emulator On-Device Run + Stripe Test Prep

**Session ID**: `9520567d-1eb6-4fff-a953-c63d65293101`
**Date**: 2026-07-15
**Branch**: master (church repo; church_mobile untouched this session)

## Goal

Execute the "on-device run" next-step from the previous session: get the
Android toolchain working, run the Flutter app on an emulator against a
live cema, and attempt the live Stripe smoke test (deferred — documented
instead).

## Work Done — Android toolchain + emulator run

- **Android SDK completed**: the SDK at `~/Library/Android/sdk` had the
  emulator, platform-tools, an android-36.1 system image, and an AVD
  (`Medium_Phone_API_36.1`) already — only cmdline-tools and license
  acceptance were missing. Installed `commandlinetools-mac-14742923` to
  `cmdline-tools/latest`, ran `yes | flutter doctor --android-licenses`.
  `flutter doctor` Android section now ✓ (Xcode still absent — no iOS).
- **cema rebuilt + started**: scratch `go.work` (`go work init . ../church`)
  in `cema/`, rebuilt, served :8088 against `church_development`; started
  redis (`--daemonize yes --dir cema/`) for sessions.
- **App run on emulator**: built/installed with
  `--dart-define=API_BASE=http://10.0.2.2:8088` (10.0.2.2 = Mac localhost
  from inside the emulator). Verified by screenshot: Home feed (both test
  sermons + Welcome article) and Sermons tab (year chips, scripture refs
  John 3:16 / Rom 15:13) render from live cema JSON. **On-device milestone
  done, emulator edition.**
- Gotchas (also saved to memory + the stripe doc):
  - `flutter run` exited 2 with "Error connecting to the service protocol"
    though the APK installed and ran fine — check
    `adb shell pidof com.rohanthewiz.church_mobile` before rebuilding.
  - First app fetch raced emulator wifi/DHCP at cold boot ("Could not
    reach the server") — relaunch the activity once network is up.
  - `adb` lives at `~/Library/Android/sdk/platform-tools/adb` (not on PATH).

## Work Done — Stripe smoke-test prep (test itself deferred by user)

- **`config/env_overrides.go`**: added `STRIPE_PUB_KEY`, `STRIPE_PRIV_KEY`,
  `STRIPE_WEBHOOK_SECRET` env overrides (env wins over yaml; same pattern
  as PG_USER/PG_WORD). Rationale: keeps secrets out of options.yml, and
  `stripe listen` mints a per-machine whsec that would otherwise force a
  yaml edit. church + cema build clean with the change.
- **Stripe CLI v1.43.8** installed (brew). `stripe listen --api-key
  sk_test_... --forward-to localhost:8088/webhooks/stripe` needs no
  interactive login.
- Recon findings: no Stripe test keys exist anywhere on this machine
  (cema options.yml has `pub_key: 'TODO'`); webhook endpoint is
  `POST /webhooks/stripe`, signature-verified, rejects when secret unset;
  `features.giving` in app-config requires **both** keys non-empty —
  that flag gates the Flutter Give tab; the local `charges` row +
  payments history are webhook-recorded, so forwarding is mandatory for
  the local-DB half of the verification.
- **`ai_docs/stripe_test_requirements.md`**: full runbook — the missing
  keys, what's staged, 6-step run procedure, verification checklist
  (PaymentSheet success → webhook 200 → charges row → history endpoint →
  guest path), test cards (4242…, decline 9995, 3DS 3155), Mac gotchas.

## Blocked / Next Steps

- **Live Stripe smoke test**: blocked only on the user's `pk_test_`/
  `sk_test_` keys — follow `ai_docs/stripe_test_requirements.md` verbatim.
- **Background-audio lock-screen controls**: still needs a physical device.
- Browser check of the responsive pass on a running site binary.
- cema + emulator left running at session end (cema task bfvc20sc2).
