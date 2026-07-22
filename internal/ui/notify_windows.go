//go:build windows

package ui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"fyne.io/fyne/v2"
)

// Windows attributes a toast notification to its AppUserModelID (Fyne passes
// appID). For an unpackaged app the only registration Windows 11 honors is a
// Start Menu shortcut carrying the AUMID property — the documented registry
// route (AppUserModelId\DisplayName) is ignored, and toasts then show the raw
// appID string instead of appName.

// startMenuPrograms is the per-user Start Menu folder, relative to %APPDATA%.
const startMenuPrograms = `Microsoft\Windows\Start Menu\Programs`

// aumidShortcutScript ensures the app's Start Menu shortcut targets the
// running executable and carries PKEY_AppUserModel_ID. It exits early when the
// shortcut already points at the current exe, so a stale target (moved or
// renamed binary) self-heals on the next launch. PowerShell with inline C#
// (for the IPropertyStore COM call) rather than native Go COM — Fyne already
// shells out to PowerShell for every toast, so the pattern and dependency are
// established.
// ponytail: Add-Type compiles C# on each run; the early exit above it makes
// that a target-changed-only cost. Port to x/sys COM vtables if it ever needs
// to be in-process.
// Sprintf order: link path, target exe, target exe, link path, AUMID.
const aumidShortcutScript = `
$shell = New-Object -ComObject WScript.Shell
$lnk = $shell.CreateShortcut('%s')
if ($lnk.TargetPath -eq '%s') { exit }
$code = @'
using System;
using System.Runtime.InteropServices;
using System.Runtime.InteropServices.ComTypes;

namespace ShortcutAumid {
    [StructLayout(LayoutKind.Sequential, Pack = 4)]
    public struct PropertyKey {
        public Guid fmtid;
        public uint pid;
        public PropertyKey(Guid f, uint p) { fmtid = f; pid = p; }
    }

    [StructLayout(LayoutKind.Explicit)]
    public struct PropVariant {
        [FieldOffset(0)] public ushort vt;
        [FieldOffset(8)] public IntPtr pointerValue;
    }

    [ComImport, InterfaceType(ComInterfaceType.InterfaceIsIUnknown), Guid("886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99")]
    public interface IPropertyStore {
        void GetCount(out uint count);
        void GetAt(uint iProp, out PropertyKey pkey);
        void GetValue(ref PropertyKey key, out PropVariant pv);
        void SetValue(ref PropertyKey key, ref PropVariant pv);
        void Commit();
    }

    [ComImport, Guid("00021401-0000-0000-C000-000000000046")]
    public class ShellLinkCoClass { }

    public static class Helper {
        [DllImport("ole32.dll", PreserveSig = false)]
        private static extern void PropVariantClear(ref PropVariant pvar);

        public static void StampAumid(string lnkPath, string aumid) {
            var link = new ShellLinkCoClass();
            ((IPersistFile)link).Load(lnkPath, 0x00000002); // STGM_READWRITE
            var store = (IPropertyStore)link;
            var key = new PropertyKey(new Guid("9F4C2855-9F79-4B39-A8D0-E1D42DE1D5F3"), 5); // PKEY_AppUserModel_ID
            var pv = new PropVariant();
            pv.vt = 31; // VT_LPWSTR
            pv.pointerValue = Marshal.StringToCoTaskMemUni(aumid);
            store.SetValue(ref key, ref pv);
            store.Commit();
            PropVariantClear(ref pv);
            ((IPersistFile)link).Save(lnkPath, true);
            Marshal.ReleaseComObject(link);
        }
    }
}
'@
Add-Type -TypeDefinition $code
$lnk.TargetPath = '%s'
$lnk.Save()
[ShortcutAumid.Helper]::StampAumid('%s', '%s')
`

// registerNotificationAppName ensures the Start Menu shortcut exists and
// targets the running executable, so toasts read "System Monitor" and the app
// launches from Windows search. The script itself skips the rewrite when the
// target already matches; on any failure notifications still work, just show
// the raw appID.
func registerNotificationAppName() {
	link := filepath.Join(os.Getenv("APPDATA"), startMenuPrograms, appName+".lnk")
	exe, err := os.Executable()
	if err != nil {
		fyne.LogError("resolve executable for start-menu shortcut", err)
		return
	}
	go runShortcutScript(fmt.Sprintf(aumidShortcutScript, link, exe, exe, link, appID))
}

// runShortcutScript executes the shortcut script via a temp .ps1 file with a
// hidden window — the same mechanism Fyne's notification delivery uses.
func runShortcutScript(script string) {
	tmp := filepath.Join(os.TempDir(), appID+"-shortcut.ps1")
	if err := os.WriteFile(tmp, []byte(script), 0o600); err != nil {
		fyne.LogError("write start-menu shortcut script", err)
		return
	}
	defer os.Remove(tmp)

	cmd := exec.Command("PowerShell", "-ExecutionPolicy", "Bypass", "-File", tmp)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Run(); err != nil {
		fyne.LogError("create start-menu shortcut", err)
	}
}
