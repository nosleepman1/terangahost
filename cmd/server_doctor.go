package cmd

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/nosleepman1/terangahost/internal/domain"
	"github.com/nosleepman1/terangahost/internal/shell"
	"github.com/nosleepman1/terangahost/internal/ui"
	"github.com/spf13/cobra"
)

var doctorServerName string

var serverDoctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Diagnostique la santé du serveur et de ses sites (services, ressources, SSL, workers, logs)",
	RunE:  func(*cobra.Command, []string) error { return runDoctor() },
}

func init() {
	serverDoctorCmd.Flags().StringVar(&doctorServerName, "name", "", "Nom du serveur à inspecter")
	_ = serverDoctorCmd.MarkFlagRequired("name")
	serverCmd.AddCommand(serverDoctorCmd)
}

type doctor struct {
	ctx      context.Context
	r        domain.Runner
	failures int
	warnings int
}

func (d *doctor) ok(format string, a ...any) {
	ui.Info("  %s %s", ui.Green("✔"), fmt.Sprintf(format, a...))
}
func (d *doctor) warn(format string, a ...any) {
	d.warnings++
	ui.Info("  %s %s", ui.Yellow("!"), fmt.Sprintf(format, a...))
}
func (d *doctor) fail(format string, a ...any) {
	d.failures++
	ui.Info("  %s %s", ui.Red("✖"), fmt.Sprintf(format, a...))
}

func (d *doctor) out(cmd string) string {
	out, _ := d.r.RunSilent(d.ctx, cmd)
	return strings.TrimSpace(out)
}

func runDoctor() error {
	store, err := openStore()
	if err != nil {
		return err
	}
	ctx, cancel := commandContext(2 * time.Minute)
	defer cancel()
	srv, err := store.FindServer(ctx, doctorServerName)
	if err != nil {
		return err
	}
	sites, err := store.ListSites(ctx, srv.ID)
	if err != nil {
		return err
	}
	r, err := connectAdmin(srv, nil)
	if err != nil {
		return err
	}
	defer r.Close()
	d := &doctor{ctx: ctx, r: r}

	ui.Info("Diagnostic de %s (%s)", ui.Cyan(srv.Name), srv.IP)

	ui.Section("Services")
	phpVersions := map[string]bool{srv.PHPVersion: true}
	for _, s := range sites {
		phpVersions[s.PHPVersion] = true
	}
	services := []string{"nginx", "supervisor", "cron", "fail2ban"}
	for v := range phpVersions {
		services = append(services, "php"+v+"-fpm")
	}
	switch srv.Database {
	case "mariadb":
		services = append(services, "mariadb")
	case "postgres":
		services = append(services, "postgresql")
	}
	if srv.WithRedis {
		services = append(services, "redis-server")
	}
	for _, svc := range services {
		if st := d.out("systemctl is-active " + shell.Quote(svc)); st == "active" {
			d.ok("%-16s actif", svc)
		} else {
			d.fail("%-16s %s", svc, orUnknown(st))
		}
	}
	if _, err := r.RunSilent(ctx, "nginx -t"); err != nil {
		d.fail("configuration Nginx invalide : %v", err)
	} else {
		d.ok("configuration Nginx valide")
	}
	if strings.Contains(d.out("ufw status"), "Status: active") {
		d.ok("pare-feu UFW actif")
	} else {
		d.fail("pare-feu UFW inactif")
	}
	if failed := d.out("systemctl --failed --no-legend --plain | awk '{print $1}'"); failed != "" {
		d.warn("unités systemd en échec : %s", strings.ReplaceAll(failed, "\n", ", "))
	}

	ui.Section("Ressources")
	if f := strings.Fields(d.out("df -h --output=pcent,avail,size / | tail -n 1")); len(f) == 3 {
		pct, _ := strconv.Atoi(strings.TrimSuffix(f[0], "%"))
		msg := fmt.Sprintf("disque : %s utilisés, %s libres sur %s", f[0], f[1], f[2])
		switch {
		case pct >= 95:
			d.fail("%s", msg)
		case pct >= 85:
			d.warn("%s", msg)
		default:
			d.ok("%s", msg)
		}
	}
	if f := strings.Fields(d.out("free -m | awk '/^Mem:/{print $2, $7}'")); len(f) == 2 {
		total, _ := strconv.Atoi(f[0])
		avail, _ := strconv.Atoi(f[1])
		msg := fmt.Sprintf("RAM : %d Mo disponibles sur %d Mo", avail, total)
		if total > 0 && avail*10 < total {
			d.warn("%s", msg)
		} else {
			d.ok("%s", msg)
		}
	}
	if swap := d.out("free -m | awk '/^Swap:/{print $3\" Mo utilisés sur \"$2\" Mo\"}'"); swap != "" {
		d.ok("swap : %s", swap)
	}
	d.ok("charge : %s", d.out("cut -d' ' -f1-3 /proc/loadavg"))

	for _, s := range sites {
		checkSite(d, s)
	}

	ui.Section("Dernières erreurs Nginx")
	if logs := d.out("tail -n 5 /var/log/nginx/error.log 2>/dev/null"); logs == "" {
		d.ok("aucune erreur récente")
	} else {
		ui.Info("%s", ui.Gray(logs))
	}

	fmt.Println()
	if d.failures > 0 {
		return fmt.Errorf("%d problème(s) critique(s) et %d avertissement(s) détectés", d.failures, d.warnings)
	}
	if d.warnings > 0 {
		ui.Warn("%d avertissement(s)", d.warnings)
	} else {
		ui.Success("Tout est en ordre")
	}
	return nil
}

func checkSite(d *doctor, s *domain.Site) {
	ui.Section("Site " + s.Domain)
	dir := shell.Quote(s.Directory)
	if rel := d.out("[ -L " + dir + "/current ] && basename \"$(readlink -f " + dir + "/current)\""); rel != "" {
		d.ok("release active : %s", rel)
	} else {
		d.warn("aucune release déployée (terangahost site deploy --domain=%s)", s.Domain)
	}
	if d.out("[ -f "+dir+"/shared/.env ] && echo yes") == "yes" {
		d.ok(".env présent")
	} else {
		d.fail(".env absent (terangahost site env push --domain=%s --file=.env.production)", s.Domain)
	}
	if s.HasSSL {
		end := d.out("openssl x509 -enddate -noout -in " + shell.Quote("/etc/letsencrypt/live/"+s.Domain+"/fullchain.pem") + " 2>/dev/null | cut -d= -f2")
		if t, err := time.Parse("Jan _2 15:04:05 2006 MST", end); err == nil {
			days := int(time.Until(t).Hours() / 24)
			switch {
			case days < 7:
				d.fail("certificat SSL expire dans %d jours", days)
			case days < 20:
				d.warn("certificat SSL expire dans %d jours (le renouvellement automatique devrait intervenir)", days)
			default:
				d.ok("certificat SSL valide encore %d jours", days)
			}
		} else {
			d.fail("certificat SSL introuvable")
		}
	}
	if s.QueueWorkers > 0 || s.WithReverb {
		status := d.out("supervisorctl status " + shell.Quote(s.ID+"-worker:*") + " " + shell.Quote(s.ID+"-reverb") + " 2>/dev/null")
		running := strings.Count(status, "RUNNING")
		expected := s.QueueWorkers
		if s.WithReverb {
			expected++
		}
		if running >= expected {
			d.ok("%d processus Supervisor en cours d'exécution", running)
		} else {
			d.fail("%d/%d processus Supervisor en cours d'exécution", running, expected)
		}
	}
	if n := d.out("tail -n 500 " + dir + "/shared/storage/logs/laravel.log 2>/dev/null | grep -c '\\.ERROR:' || true"); n != "" && n != "0" {
		d.warn("%s erreur(s) dans les 500 dernières lignes de laravel.log", n)
	}
}

func orUnknown(s string) string {
	if s == "" {
		return "inconnu"
	}
	return s
}
