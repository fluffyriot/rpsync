function toggleDropdown(id) {
    document.querySelectorAll('.dropdown-content').forEach(d => {
        if (d.id !== id) d.classList.add('hidden');
    });

    const dropdown = document.getElementById(id);
    if (dropdown) {
        dropdown.classList.toggle('hidden');
    }
}

function submitWithConfirm(form, message) {
    if (message && !window.confirm(message)) {
        return false;
    }

    const btn = form.querySelector('button[type="submit"]');
    const originalContent = btn ? btn.innerHTML : '';
    if (btn) {
        btn.disabled = true;
        btn.classList.add('loading');
        btn.innerHTML = '<i data-lucide="loader-2" class="spin"></i> Processing...';
        if (window.lucide) lucide.createIcons();
    }

    fetch(form.action, {
        method: form.method,
        body: new FormData(form)
    })
        .then(res => {
            if (res.ok) {
                window.location.reload();
            } else {
                res.text().then(text => alert("Operation failed: " + text));
                if (btn) {
                    btn.disabled = false;
                    btn.innerHTML = originalContent;
                    if (window.lucide) lucide.createIcons();
                }
            }
        })
        .catch(err => {
            console.error(err);
            alert("Error performing operation");
            if (btn) {
                btn.disabled = false;
                btn.innerHTML = originalContent;
                if (window.lucide) lucide.createIcons();
            }
        });

    return false;
}

function showReleaseNotes(version, lastSeenVersion) {
    const overlay = document.getElementById('release-notes-overlay');
    const sidebar = document.getElementById('release-notes-sidebar');
    const versionSpan = document.getElementById('release-notes-version');
    const bodyDiv = document.getElementById('release-notes-body');
    const linkBtn = document.getElementById('release-notes-link');

    if (!overlay || !sidebar) return;

    overlay.classList.add('active');
    sidebar.classList.add('active');

    if (versionSpan) versionSpan.innerText = version;

    fetch(`/api/updates/notes?version=${version}&last_seen=${lastSeenVersion}`)
        .then(res => res.json())
        .then(data => {
            if (data.error) {
                bodyDiv.innerHTML = `<div class="text-danger">Failed to load notes: ${data.error}</div>`;
                return;
            }

            if (window.marked) {
                bodyDiv.innerHTML = marked.parse(data.notes || "_No release notes available._");
            } else {
                bodyDiv.innerText = data.notes || "No release notes available.";
            }

            if (data.html_url && linkBtn) {
                linkBtn.href = data.html_url;
            }

            if (window.lucide) lucide.createIcons();
        })
        .catch(err => {
            if (bodyDiv) bodyDiv.innerHTML = `<div class="text-danger">Error fetching notes.</div>`;
            console.error(err);
        });
}

function closeReleaseNotes(appVersion) {
    const overlay = document.getElementById('release-notes-overlay');
    const sidebar = document.getElementById('release-notes-sidebar');

    if (overlay) overlay.classList.remove('active');
    if (sidebar) sidebar.classList.remove('active');

    const versionToAck = appVersion || window.rpsyncAppVersion;

    if (versionToAck) {
        fetch('/api/user/ack-version', {
            method: 'POST',
            headers: {
                'Content-Type': 'application/json'
            },
            body: JSON.stringify({ version: versionToAck })
        })
            .then(res => {
                if (!res.ok) console.error("Failed to ack version");
            })
            .catch(err => console.error(err));
    }
}

window.onclick = function (event) {
    if (!event.target.closest('.dropdown')) {
        document.querySelectorAll('.dropdown-content').forEach(d => {
            d.classList.add('hidden');
        });
    }
}

document.addEventListener("DOMContentLoaded", function () {
    if (window.lucide) lucide.createIcons();

    const toggle = document.getElementById('menu-toggle');
    const links = document.getElementById('nav-links');

    if (toggle && links) {
        toggle.addEventListener('click', function () {
            links.classList.toggle('active');
        });
    }

    if ('serviceWorker' in navigator) {
        navigator.serviceWorker.register('/static/sw.js')
            .then((registration) => {
                console.log('ServiceWorker registration successful with scope: ', registration.scope);
            })
            .catch((err) => {
                console.log('ServiceWorker registration failed: ', err);
            });
    }

    document.querySelectorAll('.dynamic-badge').forEach(el => {
        if (el.dataset.bgColor) {
            el.style.backgroundColor = el.dataset.bgColor;
        }
    });

    const syncingBadges = Array.from(document.querySelectorAll('.badge-warning'))
        .filter(el => el.textContent.trim() === 'Syncing');

    if (syncingBadges.length > 0) {
        console.log("Sync in progress detected, scheduling reload...");
        setTimeout(() => {
            window.location.reload();
        }, 5000);
    }

    const metrics = document.querySelectorAll('.formatted-metric');
    metrics.forEach(el => {
        let val = parseInt(el.textContent.trim());
        if (isNaN(val)) return;

        if (val >= 100000) {
            let shortened = (val / 1000).toFixed(1);
            if (shortened.endsWith('.0')) {
                shortened = shortened.slice(0, -2);
            }
            el.textContent = shortened + 'k';
        } else if (val >= 1000) {
            el.textContent = val.toLocaleString();
        }
    });
});
