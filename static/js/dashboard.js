// SPDX-License-Identifier: AGPL-3.0-only

document.addEventListener('DOMContentLoaded', function () {


  function formatMetric(n) {
    if (n === null || n === undefined) return '0';
    if (n >= 1000000) return (n / 1000000).toFixed(1).replace(/\.0$/, '') + 'M';
    if (n >= 1000) return (n / 1000).toFixed(1).replace(/\.0$/, '') + 'K';
    return String(n);
  }

  function networkColors() {
    try {
      return JSON.parse(document.getElementById('network-colors-data').textContent);
    } catch (e) {
      return {};
    }
  }

  fetch('/analytics/website')
    .then(r => r.json())
    .then(data => {
      if (!data || data.length === 0) return;

      Chart.defaults.color = '#a0a0a0';
      Chart.defaults.borderColor = '#333';

      const colorMap = { 'Website Visitors': '#f9ab00', 'Page Views': '#e37400' };
      const allDates = new Set();
      const datasets = [];

      data.forEach((series, index) => {
        const dataPoints = (series.points || []).map(p => {
          allDates.add(p.date);
          return { x: p.date, y: p.value };
        });
        const fallbacks = ['#f87171', '#fbbf24', '#a78bfa', '#e879f9'];
        const color = colorMap[series.label] || fallbacks[index % fallbacks.length];
        datasets.push({
          label: '  ' + series.label,
          data: dataPoints,
          borderColor: color,
          backgroundColor: color + '20',
          fill: true, tension: 0.3, borderWidth: 2, pointRadius: 3
        });
      });

      const ctx = document.getElementById('analyticsChart').getContext('2d');
      new Chart(ctx, {
        type: 'line',
        data: { labels: Array.from(allDates).sort(), datasets },
        options: {
          responsive: true, maintainAspectRatio: false,
          interaction: { mode: 'index', intersect: false },
          scales: {
            x: { type: 'category', grid: { color: 'rgba(255,255,255,0.05)' } },
            y: { beginAtZero: true, title: { display: true, text: 'Count' }, grid: { color: 'rgba(255,255,255,0.05)' } }
          },
          plugins: {
            legend: { position: 'bottom', labels: { padding: 20, usePointStyle: true } },
            tooltip: {
              backgroundColor: 'rgba(20,20,30,0.9)', titleColor: '#fff', bodyColor: '#ccc',
              padding: 10, cornerRadius: 8, displayColors: true,
              callbacks: {
                label: ctx => {
                  const l = ctx.dataset.label.trim();
                  return (l ? l + ': ' : '') + (ctx.parsed.y ?? '');
                }
              }
            }
          }
        }
      });
    })
    .catch(err => console.error('Error fetching analytics stats:', err));

  fetch('/analytics/engagement')
    .then(r => r.json())
    .then(data => {
      if (!data || data.length === 0) return;

      Chart.defaults.color = '#a0a0a0';
      Chart.defaults.borderColor = '#333';

      const colors = networkColors();
      const allDates = new Set();
      const networkGroups = {};

      data.forEach(source => {
        if (!networkGroups[source.network]) networkGroups[source.network] = new Map();
        (source.points || []).forEach(p => {
          allDates.add(p.date);
          networkGroups[source.network].set(p.date,
            (networkGroups[source.network].get(p.date) || 0) + p.likes + p.reposts);
        });
      });

      const sortedDates = Array.from(allDates).sort();
      const datasets = Object.keys(networkGroups).map(network => {
        const color = colors[network] || '#9167e4';
        return {
          label: ' ' + network,
          data: sortedDates.map(d => networkGroups[network].get(d) || 0),
          backgroundColor: color, borderColor: color, borderWidth: 1, fill: true
        };
      });

      const totalLabelsPlugin = {
        id: 'totalLabels',
        afterDatasetsDraw: chart => {
          if (window.innerWidth < 768) return;
          const { ctx, scales: { x, y } } = chart;
          ctx.save();
          ctx.font = 'bold 11px sans-serif';
          ctx.fillStyle = '#a0a0a0';
          ctx.textAlign = 'center';
          ctx.textBaseline = 'bottom';
          chart.data.labels.forEach((_, i) => {
            let total = 0, hasData = false;
            chart.data.datasets.forEach((ds, di) => {
              if (!chart.isDatasetVisible(di)) return;
              const v = ds.data[i];
              if (typeof v === 'number') { total += v; if (v > 0) hasData = true; }
            });
            if (hasData && total > 0) {
              const meta = chart.getDatasetMeta(0);
              if (meta.data[i]) ctx.fillText(total, meta.data[i].x, y.getPixelForValue(total) - 5);
            }
          });
          ctx.restore();
        }
      };

      const ctx = document.getElementById('engagementChart').getContext('2d');
      new Chart(ctx, {
        type: 'bar',
        data: { labels: sortedDates, datasets },
        plugins: [totalLabelsPlugin],
        options: {
          responsive: true, maintainAspectRatio: false,
          interaction: { mode: 'index', intersect: false },
          scales: {
            x: { stacked: true, type: 'category', grid: { color: 'rgba(255,255,255,0.05)' } },
            y: { stacked: true, beginAtZero: true, grace: '5%', title: { display: true, text: 'Posts Engagement' }, grid: { color: 'rgba(255,255,255,0.05)' } }
          },
          plugins: {
            legend: { position: 'bottom', labels: { padding: 20, usePointStyle: true } },
            tooltip: {
              backgroundColor: 'rgba(20,20,30,0.9)', titleColor: '#fff', bodyColor: '#ccc',
              padding: 10, cornerRadius: 8, displayColors: true,
              callbacks: {
                label: ctx => {
                  const l = ctx.dataset.label || '';
                  return (l ? l + ': ' : '') + (ctx.parsed.y ?? '');
                }
              }
            }
          }
        }
      });
    })
    .catch(err => console.error('Error fetching engagement stats:', err));

  function setStatValue(cardId, value, suffix) {
    const card = document.getElementById(cardId);
    if (!card) return;
    card.querySelector('.stat-value').textContent = formatMetric(value) + (suffix || '');
  }

  function buildSourceTile(src, colors) {
    const tile = document.createElement('div');
    tile.className = 'source-tile';

    const header = document.createElement('div');
    header.className = 'source-tile-header';

    const img = document.createElement('img');
    img.src = '/static/images/sources/' + src.network.toLowerCase() + '_logo.svg';
    img.alt = src.network;
    img.className = 'source-logo';

    const placeholder = document.createElement('div');
    placeholder.className = 'source-logo-placeholder hidden';
    placeholder.dataset.color = colors[src.network] || '#9167e4';
    placeholder.textContent = src.network.charAt(0).toUpperCase();
    img.addEventListener('error', function () {
      img.style.display = 'none';
      placeholder.classList.remove('hidden');
      placeholder.style.display = 'flex';
      placeholder.style.backgroundColor = placeholder.dataset.color;
    });

    const nameWrap = document.createElement('div');
    nameWrap.className = 'min-w-0';

    const link = document.createElement('a');
    link.href = src.profile_url;
    link.target = '_blank';
    link.rel = 'noopener noreferrer';
    link.className = 'source-name';
    link.title = src.username;
    link.textContent = src.username.replace(/^@/, '');

    const netLabel = document.createElement('div');
    netLabel.className = 'source-network';
    netLabel.textContent = src.network;

    nameWrap.append(link, netLabel);
    header.append(img, placeholder, nameWrap);
    tile.appendChild(header);

    const statsDiv = document.createElement('div');
    statsDiv.className = 'source-stats';

    if (src.engagement_supported || src.views_supported) {
      const stat = document.createElement('div');
      stat.className = 'source-stat';

      const valSpan = document.createElement('span');
      valSpan.className = 'source-stat-value';

      if (src.engagement_supported) {
        const s = document.createElement('span');
        s.textContent = formatMetric(src.total_interactions);
        valSpan.appendChild(s);
      }
      if (src.engagement_supported && src.views_supported) {
        valSpan.appendChild(document.createTextNode(' / '));
      }
      if (src.views_supported) {
        const s = document.createElement('span');
        s.textContent = formatMetric(src.total_views);
        valSpan.appendChild(s);
      }

      const lbl = document.createElement('span');
      lbl.className = 'source-stat-label';
      if (src.engagement_supported && src.views_supported) lbl.textContent = 'Engag. / Views';
      else if (src.engagement_supported) lbl.textContent = 'Engagement';
      else lbl.textContent = 'Views';

      stat.append(valSpan, lbl);
      statsDiv.appendChild(stat);
    }

    if (src.followers_tracked) {
      const stat = document.createElement('div');
      stat.className = 'source-stat source-stat-right';

      const valSpan = document.createElement('span');
      valSpan.className = 'source-stat-value';
      valSpan.textContent = formatMetric(src.followers_count);

      const lbl = document.createElement('span');
      lbl.className = 'source-stat-label';
      lbl.textContent = 'Followers';

      stat.append(valSpan, lbl);
      statsDiv.appendChild(stat);
    }

    tile.appendChild(statsDiv);
    return tile;
  }

  function renderTopSources(sources, colors) {
    const container = document.getElementById('top-sources-container');
    if (!container) return;
    container.innerHTML = '';
    container.classList.remove('top-sources-loading');

    if (!sources || sources.length === 0) {
      container.remove();
      return;
    }

    sources.forEach(src => container.appendChild(buildSourceTile(src, colors)));

    const showAllBtn = document.createElement('button');
    showAllBtn.id = 'show-all-sources-btn';
    showAllBtn.className = 'btn btn-secondary w-full col-span-full hidden';
    const icon = document.createElement('i');
    icon.setAttribute('data-lucide', 'chevrons-down');
    showAllBtn.append(icon, document.createTextNode(' Show All'));
    container.appendChild(showAllBtn);

    if (window.lucide) lucide.createIcons();

    const tiles = container.querySelectorAll('.source-tile');
    const firstTileTop = tiles[0].offsetTop;
    let firstRowCount = 0;
    for (const tile of tiles) {
      if (tile.offsetTop === firstTileTop) firstRowCount++;
      else break;
    }
    const visibleCount = Math.max(firstRowCount, 2);
    let hasHidden = false;
    tiles.forEach((t, i) => {
      if (i >= visibleCount) { t.classList.add('hidden'); hasHidden = true; }
    });
    if (hasHidden) {
      showAllBtn.classList.remove('hidden');
      showAllBtn.addEventListener('click', () => {
        container.querySelectorAll('.source-tile.hidden').forEach(t => t.classList.remove('hidden'));
        showAllBtn.remove();
      });
    }
  }

  function renderRecentLogs(logs) {
    const content = document.getElementById('recent-logs-content');
    if (!content) return;
    content.innerHTML = '';

    if (!logs || logs.length === 0) {
      const icon = document.createElement('i');
      icon.setAttribute('data-lucide', 'check-circle');
      icon.className = 'icon-xl opacity-50 text-success';
      const p = document.createElement('p');
      p.textContent = 'No recent errors found.';
      content.className = 'text-center p-8 text-muted';
      content.append(icon, p);
      if (window.lucide) lucide.createIcons();
      return;
    }

    content.className = '';

    const wrapper = document.createElement('div');
    wrapper.style.overflowX = 'auto';

    const table = document.createElement('table');
    table.className = 'w-full border-collapse text-sm';

    const thead = document.createElement('thead');
    const headRow = document.createElement('tr');
    headRow.className = 'text-left border-b';
    ['Time', 'Source/Target', 'Message'].forEach(text => {
      const th = document.createElement('th');
      th.className = 'p-3';
      th.textContent = text;
      headRow.appendChild(th);
    });
    thead.appendChild(headRow);
    table.appendChild(thead);

    const tbody = document.createElement('tbody');
    logs.forEach(log => {
      const tr = document.createElement('tr');
      tr.className = 'border-b';

      const tdTime = document.createElement('td');
      tdTime.className = 'p-3 whitespace-nowrap text-muted';
      tdTime.textContent = typeof relativeTime === 'function' ? relativeTime(log.created_at) : log.created_at;
      if (typeof formatUTCTooltip === 'function') tdTime.title = formatUTCTooltip(log.created_at);

      const tdSource = document.createElement('td');
      tdSource.className = 'p-3';
      if (log.source_network) {
        const wrap = document.createElement('div');
        wrap.className = 'flex items-center gap-2';
        const badge = document.createElement('span');
        badge.className = 'badge badge-neutral';
        badge.textContent = log.source_network;
        const name = document.createElement('span');
        name.textContent = log.source_username;
        wrap.append(badge, name);
        tdSource.appendChild(wrap);
      } else if (log.target_type) {
        const wrap = document.createElement('div');
        wrap.className = 'flex items-center gap-2';
        const badge = document.createElement('span');
        badge.className = 'badge badge-neutral';
        badge.textContent = 'Target';
        const name = document.createElement('span');
        name.textContent = log.target_type;
        wrap.append(badge, name);
        tdSource.appendChild(wrap);
      } else {
        const s = document.createElement('span');
        s.className = 'text-muted';
        s.textContent = 'System';
        tdSource.appendChild(s);
      }

      const tdMsg = document.createElement('td');
      tdMsg.className = 'p-3 text-danger';
      const msgWrap = document.createElement('div');
      msgWrap.className = 'flex justify-between items-start gap-2';
      const msgSpan = document.createElement('span');
      msgSpan.textContent = log.message;

      const form = document.createElement('form');
      form.method = 'POST';
      form.action = '/logs/dismiss';
      form.className = 'm-0';
      const input = document.createElement('input');
      input.type = 'hidden';
      input.name = 'id';
      input.value = log.id;
      const dismissBtn = document.createElement('button');
      dismissBtn.type = 'submit';
      dismissBtn.className = 'btn btn-sm btn-ghost btn-icon';
      dismissBtn.title = 'Dismiss';
      const xIcon = document.createElement('i');
      xIcon.setAttribute('data-lucide', 'x');
      xIcon.className = 'icon-sm';
      dismissBtn.appendChild(xIcon);
      form.append(input, dismissBtn);

      msgWrap.append(msgSpan, form);
      tdMsg.appendChild(msgWrap);

      tr.append(tdTime, tdSource, tdMsg);
      tbody.appendChild(tr);
    });

    table.appendChild(tbody);
    wrapper.appendChild(table);
    content.appendChild(wrapper);

    if (window.lucide) lucide.createIcons();
  }

  function renderComparisonChart(ctxId, currentData, previousData, primaryColor) {
    const canvasEl = document.getElementById(ctxId);
    if (!canvasEl) return;
    const ctx = canvasEl.getContext('2d');
    const dayLabels = Array.from({ length: 7 }, (_, i) => 'Day ' + (i + 1));

    new Chart(ctx, {
      type: 'line',
      data: {
        labels: dayLabels,
        datasets: [
          {
            label: '  Last 7 Days',
            data: currentData.map(p => p.value),
            borderColor: primaryColor, backgroundColor: primaryColor + '20',
            fill: true, tension: 0.3, borderWidth: 2, pointRadius: 2
          },
          {
            label: '  Previous 7 Days',
            data: previousData.map(p => p.value),
            borderColor: '#6B7280', backgroundColor: 'transparent',
            borderDash: [5, 5], fill: false, tension: 0.3, borderWidth: 2, pointRadius: 2
          }
        ]
      },
      options: {
        responsive: true, maintainAspectRatio: false,
        interaction: { mode: 'index', intersect: false },
        scales: {
          x: { grid: { display: false }, ticks: { display: true, maxTicksLimit: 7 } },
          y: { beginAtZero: false, grid: { color: 'rgba(255,255,255,0.05)' } }
        },
        plugins: {
          legend: { position: 'bottom', labels: { usePointStyle: true, boxWidth: 8 } },
          tooltip: {
            backgroundColor: 'rgba(20,20,30,0.9)', titleColor: '#fff', bodyColor: '#ccc',
            padding: 10, cornerRadius: 8,
            callbacks: {
              label: context => {
                const idx = context.dataIndex;
                const val = context.parsed.y;
                const pool = context.datasetIndex === 0 ? currentData : previousData;
                const dateStr = pool[idx] ? ' (' + pool[idx].date + ')' : '';
                return context.dataset.label.trim() + ': ' + val + dateStr;
              }
            }
          }
        }
      }
    });
  }

  const colors = networkColors();

  fetch('/api/dashboard/stats')
    .then(r => r.json())
    .then(data => {
      setStatValue('stat-sync-errors', data.sync_errors_30d);
      if (data.sync_errors_30d > 0) {
        document.getElementById('stat-sync-errors').classList.add('has-errors');
      }
      setStatValue('stat-active-sources', data.active_sources);
      setStatValue('stat-active-targets', data.active_targets);
      setStatValue('stat-total-posts', data.total_posts);
      setStatValue('stat-total-likes', data.total_likes);
      setStatValue('stat-total-shares', data.total_shares);
      setStatValue('stat-total-views', data.total_views);
      setStatValue('stat-total-visitors', data.total_visitors);
      setStatValue('stat-total-page-views', data.total_page_views);
      setStatValue('stat-avg-session', data.average_website_session, 's');
      renderTopSources(data.top_sources, colors);
    })
    .catch(err => {
      console.error('Error fetching dashboard stats:', err);
      const topSources = document.getElementById('top-sources-container');
      if (topSources) topSources.remove();
    });

  fetch('/analytics/summary')
    .then(r => r.json())
    .then(data => {
      if (data.engagement) renderComparisonChart('engagementComparisonChart', data.engagement.current_period, data.engagement.previous_period, '#9167e4');
      if (data.followers) renderComparisonChart('followersComparisonChart', data.followers.current_period, data.followers.previous_period, '#9167e4');
    })
    .catch(err => console.error('Error fetching dashboard summary:', err));

  fetch('/api/dashboard/logs')
    .then(r => r.json())
    .then(logs => renderRecentLogs(logs))
    .catch(err => {
      console.error('Error fetching recent logs:', err);
      const content = document.getElementById('recent-logs-content');
      if (content) {
        content.className = 'text-center p-8 text-muted';
        const p = document.createElement('p');
        p.className = 'text-danger';
        p.textContent = 'Failed to load recent logs.';
        content.innerHTML = '';
        content.appendChild(p);
      }
    });

});
