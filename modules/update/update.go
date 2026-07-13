package update

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/celestix/gotgproto/ext"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/plumbing/transport/ssh"
	"github.com/hikari-work/userbot/modules/manager"
	"github.com/hikari-work/userbot/utils"
)

func init() {
	manager.Register(&manager.Module{
		Name:        "Update",
		Description: "Mengupdate repositori dari remote upstream dan merestart bot",
		Commands:    []string{"update"},
		OnlyOut:     true,
		Handler:     updateHandler,
		Help:        updateHelp,
	})
}

func updateHelp() string {
	return "Format:\n<code>.update</code>\n\nFungsi: Mengambil pembaruan terbaru dari GitHub upstream, mencatat git log, dan merestart bot."
}

func updateHandler(ctx *ext.Context, update *ext.Update) error {
	msg := update.EffectiveMessage
	if msg == nil {
		return nil
	}

	uChat := update.EffectiveChat()
	chatID := uChat.GetID()

	// 1. Send initial status
	statusMsg, err := ctx.Reply(update, ext.ReplyTextString("⏳ <b>Sedang memeriksa pembaruan di upstream repository...</b>"), nil)
	if err != nil {
		return err
	}

	// 2. Open Git repository
	repo, err := git.PlainOpen(".")
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, chatID, statusMsg.ID, fmt.Sprintf("❌ Gagal membuka repositori git lokal: %v", err))
		return err
	}

	// 3. Get remote origin URL to determine authentication method
	remote, err := repo.Remote("origin")
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, chatID, statusMsg.ID, fmt.Sprintf("❌ Gagal mendapatkan remote 'origin': %v", err))
		return err
	}

	var remoteURL string
	if len(remote.Config().URLs) > 0 {
		remoteURL = remote.Config().URLs[0]
	}

	// Resolve appropriate AuthMethod
	auth, err := getAuthMethod(remoteURL)
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, chatID, statusMsg.ID, fmt.Sprintf("❌ Gagal mempersiapkan autentikasi SSH: %v", err))
		return err
	}

	// 4. Get current HEAD before update
	ref, err := repo.Head()
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, chatID, statusMsg.ID, fmt.Sprintf("❌ Gagal mendapatkan HEAD commit saat ini: %v", err))
		return err
	}
	oldHash := ref.Hash()

	// 5. Get worktree
	w, err := repo.Worktree()
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, chatID, statusMsg.ID, fmt.Sprintf("❌ Gagal mendapatkan git worktree: %v", err))
		return err
	}

	// 6. Pull from Remote (origin main)
	_, _ = utils.EditMessageHTML(ctx, chatID, statusMsg.ID, "📥 <b>Menarik perubahan dari upstream (git pull)...</b>")

	pullOpts := &git.PullOptions{
		RemoteName:    "origin",
		ReferenceName: plumbing.NewBranchReferenceName("main"),
		SingleBranch:  true,
		Force:         true,
		Auth:          auth,
	}

	err = w.Pull(pullOpts)
	if err != nil {
		if err == git.NoErrAlreadyUpToDate {
			_, _ = utils.EditMessageHTML(ctx, chatID, statusMsg.ID, "✅ <b>Aplikasi sudah berada di versi terbaru (Up-to-date).</b>")
			return nil
		}
		_, _ = utils.EditMessageHTML(ctx, chatID, statusMsg.ID, fmt.Sprintf("❌ Gagal melakukan git pull: %v<br/><i>Tip: Pastikan URL remote origin bertipe HTTPS jika publik, atau pasang SSH Key / Agent jika privat.</i>", err))
		return err
	}

	// 7. Get new HEAD commit
	newRef, err := repo.Head()
	if err != nil {
		_, _ = utils.EditMessageHTML(ctx, chatID, statusMsg.ID, fmt.Sprintf("❌ Gagal mendapatkan HEAD baru setelah pull: %v", err))
		return err
	}
	newHash := newRef.Hash()

	// 8. Get commits log between old and new commit
	var logs []string
	cIter, err := repo.Log(&git.LogOptions{From: newHash})
	if err == nil {
		count := 0
		_ = cIter.ForEach(func(c *object.Commit) error {
			if c.Hash == oldHash || count >= 20 {
				return storer.ErrStop
			}
			shortHash := c.Hash.String()[:7]
			msgSummary := strings.Split(c.Message, "\n")[0]
			author := c.Author.Name
			date := c.Author.When.Format("02-01-2006 15:04")

			logs = append(logs, fmt.Sprintf("🔹 <code>%s</code> - %s (%s) <i>by %s</i>", shortHash, msgSummary, date, author))
			count++
			return nil
		})
	}

	changelogText := "Tidak ada detail log commit baru."
	if len(logs) > 0 {
		changelogText = strings.Join(logs, "\n")
	}

	reportText := fmt.Sprintf(
		"🚀 <b>Pembaruan Berhasil Diunduh!</b>\n\n"+
			"📌 <b>Sebelumnya:</b> <code>%s</code>\n"+
			"📌 <b>Sekarang:</b> <code>%s</code>\n\n"+
			"📝 <b>Changelog Pembaruan:</b>\n%s\n\n"+
			"⚙️ <i>Bot akan mati sekarang dan melakukan kompilasi ulang (rebuild) di Docker. Tunggu beberapa saat untuk menyala kembali...</i>",
		oldHash.String()[:7],
		newHash.String()[:7],
		changelogText,
	)

	_, _ = utils.EditMessageHTML(ctx, chatID, statusMsg.ID, reportText)

	// 9. Graceful exit to let Docker Entrypoint compile and restart
	go func() {
		time.Sleep(2 * time.Second)
		os.Exit(0)
	}()

	return nil
}

// getAuthMethod checks if SSH auth is required and returns the appropriate AuthMethod.
func getAuthMethod(remoteURL string) (transport.AuthMethod, error) {
	if !strings.HasPrefix(remoteURL, "git@") && !strings.HasPrefix(remoteURL, "ssh://") {
		return nil, nil // No auth required (usually anonymous HTTPS pull)
	}

	// If SSH_AUTH_SOCK is present, let go-git's default agent loader handle it
	if os.Getenv("SSH_AUTH_SOCK") != "" {
		return nil, nil
	}

	// Try reading private key from SSH_PRIVATE_KEY environment variable
	if privateKey := os.Getenv("SSH_PRIVATE_KEY"); privateKey != "" {
		passphrase := os.Getenv("SSH_KEY_PASSWORD")
		publicKeys, err := ssh.NewPublicKeys("git", []byte(privateKey), passphrase)
		if err == nil {
			return publicKeys, nil
		}
	}

	// Try scanning standard local private key paths in the workspace/home folder
	homeDir, _ := os.UserHomeDir()
	keyPaths := []string{
		filepath.Join(homeDir, ".ssh", "id_rsa"),
		filepath.Join(homeDir, ".ssh", "id_ed25519"),
		"/app/.ssh/id_rsa",
		"/app/.ssh/id_ed25519",
		"/app/id_rsa",
	}

	for _, kp := range keyPaths {
		if _, err := os.Stat(kp); err == nil {
			passphrase := os.Getenv("SSH_KEY_PASSWORD")
			publicKeys, err := ssh.NewPublicKeysFromFile("git", kp, passphrase)
			if err == nil {
				return publicKeys, nil
			}
		}
	}

	return nil, nil
}
