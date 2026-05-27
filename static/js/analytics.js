document.addEventListener("DOMContentLoaded", function () {
    const colors = {
        primary: '#9167e4',
        primaryLight: '#b39ddb',
        primaryDark: '#5e35b1',
        accent: '#d1c4e9',
        text: '#a0a0a0',
        grid: 'rgba(255,255,255,0.05)',
        highContrast: ['#9167e4', '#ef4444', '#f59e0b', '#10b981', '#3b82f6', '#ec4899', '#6366f1', '#8b5cf6']
    };

    Chart.defaults.color = colors.text;
    Chart.defaults.borderColor = colors.grid;
    Chart.defaults.font.family = "'Inter', system-ui, sans-serif";

    const loadedTabs = new Set();
    const chartInstances = {};
    const filterState = { startDate: '', endDate: '', postTypes: null, mode: 'likes', tagIds: null };

    function isViewsMode() { return filterState.mode === 'views'; }
    function engagementLabel() { return isViewsMode() ? 'Avg Views' : 'Avg Likes'; }

    function buildFilterParams() {
        const params = new URLSearchParams();
        if (filterState.startDate) params.set('start_date', filterState.startDate);
        if (filterState.endDate) params.set('end_date', filterState.endDate);
        if (filterState.postTypes !== null && filterState.postTypes.length > 0) {
            params.set('post_types', filterState.postTypes.join(','));
        }
        if (filterState.tagIds !== null && filterState.tagIds.length > 0) {
            params.set('tag_ids', filterState.tagIds.join(','));
        }
        if (filterState.mode === 'views') params.set('mode', 'views');
        return params;
    }

    function getFilteredUrl(base) {
        const params = buildFilterParams();
        const qs = params.toString();
        return qs ? `${base}?${qs}` : base;
    }

    function applyGlobalFilters() {
        Object.values(chartInstances).forEach(c => c.destroy());
        Object.keys(chartInstances).forEach(k => delete chartInstances[k]);
        loadedTabs.clear();
        const activeTabBtn = document.querySelector('.tab-btn.active');
        if (activeTabBtn) loadTab(activeTabBtn.dataset.tab);
    }

    function loadTab(tabName) {
        if (loadedTabs.has(tabName) && tabName !== 'wordcloud') return;

        if (loadedTabs.has(tabName)) return;
        loadedTabs.add(tabName);

        switch (tabName) {
            case 'content':
                loadHashtags();
                loadTopInteracted();
                loadPerformanceDeviation();
                loadEngagementVelocity();
                break;
            case 'engagement':
                loadPostTypes();
                loadNetworkEfficiency();
                loadMentions();
                loadEngagementRate();
                loadFollowRatio();
                loadCollaborations();
                break;
            case 'timing':
                loadTiming();
                loadPostingConsistency();
                break;
            case 'wordcloud':
                loadWordCloud();
                break;
            case 'website':
                loadWebsiteStats();
                break;
            case 'tags':
                loadTagAnalytics();
                break;
        }
    }

    const tabs = document.querySelectorAll('.tab-btn');
    const contents = document.querySelectorAll('.tab-content');

    const urlParams = new URLSearchParams(window.location.search);
    const activeTab = urlParams.get('tab') || 'content';

    function setActiveTab(tabName) {
        tabs.forEach(t => {
            if (t.dataset.tab === tabName) t.classList.add('active');
            else t.classList.remove('active');
        });
        contents.forEach(c => {
            if (c.id === `${tabName}-tab`) {
                c.classList.remove('hidden');
                c.classList.add('active');
            } else {
                c.classList.add('hidden');
                c.classList.remove('active');
            }
        });
        loadTab(tabName);
    }

    tabs.forEach(tab => {
        tab.addEventListener('click', () => {
            const tabName = tab.dataset.tab;
            setActiveTab(tabName);

            const url = new URL(window.location);
            url.searchParams.set('tab', tabName);
            window.history.pushState({}, '', url);
        });
    });

    setActiveTab(activeTab);

    function showEmptyState(containerEl) {
        containerEl.querySelectorAll('canvas').forEach(c => { c.style.display = 'none'; });
        if (containerEl.querySelector('.chart-empty-state')) return;
        const div = document.createElement('div');
        div.className = 'chart-empty-state';
        const icon = document.createElement('i');
        icon.dataset.lucide = 'bar-chart-2';
        div.appendChild(icon);
        const span = document.createElement('span');
        span.textContent = 'Not enough data';
        div.appendChild(span);
        containerEl.appendChild(div);
        if (window.lucide) lucide.createIcons();
    }

    function hideEmptyState(containerEl) {
        containerEl.querySelectorAll('canvas').forEach(c => { c.style.display = ''; });
        containerEl.querySelectorAll('.chart-empty-state').forEach(el => el.remove());
    }

    function emptyChartState(canvasId) {
        const canvas = document.getElementById(canvasId);
        if (!canvas) return;
        showEmptyState(canvas.closest('.chart-container') || canvas.parentElement);
    }

    function createChart(ctxId, type, data, options = {}) {
        if (chartInstances[ctxId]) {
            chartInstances[ctxId].destroy();
            delete chartInstances[ctxId];
        }
        const canvasEl = document.getElementById(ctxId);
        hideEmptyState(canvasEl.closest('.chart-container') || canvasEl.parentElement);
        const ctx = canvasEl.getContext('2d');
        const chart = new Chart(ctx, {
            type: type,
            data: data,
            options: {
                responsive: true,
                maintainAspectRatio: false,
                ...options
            }
        });
        chartInstances[ctxId] = chart;
        return chart;
    }

    function loadWordCloud() {
        fetch(getFilteredUrl('/analytics/data/wordcloud'))
            .then(res => res.json())
            .then(data => {
                renderWordCloud(data, 'wordCloudCanvas', 'word-cloud-container', 'usage_count');
            })
            .catch(err => console.error(err));

        fetch(getFilteredUrl('/analytics/data/wordcloud/engagement'))
            .then(res => res.json())
            .then(data => {
                renderWordCloud(data, 'wordCloudEngagementCanvas', 'word-cloud-engagement-container', 'avg_engagement');
            })
            .catch(err => console.error(err));
    }

    function renderWordCloud(data, canvasId, containerId, valueKey) {
        const container = document.getElementById(containerId);
        if (!data || !Array.isArray(data) || data.length === 0) {
            if (container) showEmptyState(container);
            return;
        }
        const canvas = document.getElementById(canvasId);

        if (!container || container.clientWidth === 0) return;

        canvas.width = container.clientWidth;
        canvas.height = container.clientHeight;

        let minVal = Infinity;
        let maxVal = -Infinity;
        data.forEach(item => {
            const val = parseFloat(item[valueKey]);
            if (!isNaN(val)) {
                if (val < minVal) minVal = val;
                if (val > maxVal) maxVal = val;
            }
        });

        if (minVal === Infinity) return;

        const valRange = maxVal - minVal;
        const widthFactor = container.clientWidth / 6;
        const maxFontSize = Math.max(60, Math.min(widthFactor, 200));
        const minFontSize = Math.max(14, maxFontSize / 10);

        const list = data.map(item => {
            const val = parseFloat(item[valueKey]);
            if (isNaN(val)) return null;

            if (valRange > 0) {
                const logMin = Math.log(minVal || 1);
                const logMax = Math.log(maxVal || 1);
                if (logMax - logMin > 0) {
                    normalized = (Math.log(val || 1) - logMin) / (logMax - logMin);
                } else {
                    normalized = 0.5;
                }
            }
            const size = minFontSize + (normalized * (maxFontSize - minFontSize));

            return [item.word, size];
        }).filter(item => item !== null);
        const ctx = canvas.getContext('2d');
        ctx.clearRect(0, 0, canvas.width, canvas.height);

        WordCloud(canvas, {
            list: list,
            gridSize: Math.round(8 * canvas.width / 1024),
            weightFactor: (size) => size,
            fontFamily: 'Inter, system-ui, sans-serif',
            color: (word, weight) => {
                const intensity = (weight - minFontSize) / (maxFontSize - minFontSize);
                if (intensity > 0.8) return '#9167e4';
                if (intensity > 0.6) return '#b39ddb';
                if (intensity > 0.4) return '#d1c4e9';
                if (intensity > 0.2) return '#e0e0e0';
                return colors.text;
            },
            rotateRatio: 0,
            backgroundColor: 'transparent',
            shrinkToFit: true,
            drawOutOfBound: false,
            origin: [canvas.width / 2, canvas.height / 2],
            hover: (item, dimension, event) => {
                if (item) {
                    const word = item[0];
                    const dataItem = data.find(d => d.word === word);
                    if (dataItem) {
                        let text = `${word}: ${dataItem.usage_count} uses`;
                        if (dataItem.avg_engagement !== undefined) {
                            const avg = parseFloat(dataItem.avg_engagement).toFixed(1);
                            text += `, ${avg} avg engagement (${dataItem.total_engagement} total)`;
                        }
                        showTooltip(event, text);
                    }
                } else {
                    hideTooltip();
                }
            }
        });
    }

    function loadHashtags() {
        fetch(getFilteredUrl('/analytics/data/hashtags'))
            .then(res => res.json())
            .then(data => {
                if (!data || !Array.isArray(data) || data.length === 0) { emptyChartState('hashtagsChart'); return; }
                const engKey = isViewsMode() ? 'avg_views' : 'avg_likes';
                const engLbl = engagementLabel();
                const sorted = [...data].sort((a, b) => b[engKey] - a[engKey]);
                createChart('hashtagsChart', 'bar', {
                    labels: sorted.map(d => d.tag),
                    datasets: [{
                        label: engLbl,
                        data: sorted.map(d => d[engKey]),
                        backgroundColor: colors.primary,
                        yAxisID: 'y',
                        order: 2
                    },
                    {
                        label: 'Usage Count',
                        data: sorted.map(d => d.usage_count),
                        borderColor: colors.accent,
                        backgroundColor: colors.accent,
                        type: 'line',
                        yAxisID: 'y1',
                        order: 1
                    }]
                }, {
                    scales: {
                        y: { position: 'left', title: { display: true, text: 'Posts Count' } },
                        y1: { position: 'right', title: { display: true, text: engLbl }, grid: { drawOnChartArea: false } }
                    },
                    plugins: {
                        legend: {
                            labels: {
                                usePointStyle: true,
                                padding: 20
                            }
                        }
                    }
                });
            });
    }

    function loadMentions() {
        fetch(getFilteredUrl('/analytics/data/mentions'))
            .then(res => res.json())
            .then(data => {
                if (!data || !Array.isArray(data) || data.length === 0) { emptyChartState('mentionsChart'); return; }
                const engKey = isViewsMode() ? 'avg_views' : 'avg_likes';
                const engLbl = engagementLabel();
                const sorted = [...data].sort((a, b) => b[engKey] - a[engKey]);
                createChart('mentionsChart', 'bar', {
                    labels: sorted.map(d => d.mention),
                    datasets: [{
                        label: engLbl,
                        data: sorted.map(d => d[engKey]),
                        backgroundColor: colors.primary,
                        yAxisID: 'y',
                        order: 2
                    }, {
                        label: 'Usage Count',
                        data: sorted.map(d => d.usage_count),
                        borderColor: colors.accent,
                        backgroundColor: colors.accent,
                        type: 'line',
                        yAxisID: 'y1',
                        order: 1
                    }]
                }, {
                    scales: {
                        y: { position: 'left', title: { display: true, text: engLbl } },
                        y1: { position: 'right', title: { display: true, text: 'Usage Count' }, grid: { drawOnChartArea: false } }
                    },
                    plugins: {
                        legend: {
                            labels: {
                                usePointStyle: true,
                                padding: 20
                            }
                        }
                    }
                });
            });
    }

    function loadPostTypes() {
        fetch(getFilteredUrl('/analytics/data/types'))
            .then(res => res.json())
            .then(data => {
                if (!data || !Array.isArray(data) || data.length === 0) { emptyChartState('postTypesChart'); return; }
                const engKey = isViewsMode() ? 'avg_views' : 'avg_likes';
                const engLbl = engagementLabel().toLowerCase();
                createChart('postTypesChart', 'doughnut', {
                    labels: data.map(d => ' ' + d.post_type),
                    datasets: [{
                        data: data.map(d => d[engKey]),
                        backgroundColor: colors.highContrast
                    }]
                }, {
                    plugins: {
                        legend: {
                            position: 'right',
                            labels: { padding: 20, usePointStyle: true }
                        },
                        tooltip: {
                            callbacks: {
                                label: function (context) {
                                    const label = context.label || '';
                                    const value = context.raw || 0;
                                    const count = data[context.dataIndex].post_count;
                                    return ` ${label}: ${value} ${engLbl} (${count} posts)`;
                                }
                            }
                        }
                    },
                    layout: { padding: 20 }
                });
            });
    }

    function loadNetworkEfficiency() {
        fetch(getFilteredUrl('/analytics/data/networks'))
            .then(res => res.json())
            .then(data => {
                if (!data || !Array.isArray(data) || data.length === 0) { emptyChartState('networkEfficiencyChart'); return; }
                const engKey = isViewsMode() ? 'avg_views' : 'avg_likes';
                const engLbl = engagementLabel();
                const sorted = [...data].sort((a, b) => b[engKey] - a[engKey]);
                createChart('networkEfficiencyChart', 'bar', {
                    labels: sorted.map(d => d.network),
                    datasets: [{
                        label: `${engLbl} per Post`,
                        data: sorted.map(d => d[engKey]),
                        backgroundColor: colors.primary,
                        yAxisID: 'y',
                        order: 2
                    }, {
                        label: 'Post Count',
                        data: sorted.map(d => d.post_count),
                        borderColor: colors.accent,
                        backgroundColor: colors.accent,
                        type: 'line',
                        yAxisID: 'y1',
                        order: 1
                    }]
                }, {
                    indexAxis: 'x',
                    scales: {
                        y: { position: 'left', title: { display: true, text: engLbl } },
                        y1: { position: 'right', title: { display: true, text: 'Post Count' }, grid: { drawOnChartArea: false } }
                    },
                    plugins: {
                        legend: {
                            labels: {
                                usePointStyle: true,
                                padding: 20
                            }
                        }
                    }
                });
            });
    }

    function loadTiming() {
        fetch(getFilteredUrl('/analytics/data/time'))
            .then(res => res.json())
            .then(data => {
                if (!data) return;
                const engKey = isViewsMode() ? 'avg_views' : 'avg_likes';
                const header = document.getElementById('timingCardHeader');
                if (header) header.textContent = `Best Time to Post (${engagementLabel()})`;
                renderHeatmap(data, engKey);
            });
    }

    let tooltipEl = null;

    function getTooltip() {
        if (!tooltipEl) {
            tooltipEl = document.createElement('div');
            tooltipEl.className = 'analytics-tooltip';
            document.body.appendChild(tooltipEl);
        }
        return tooltipEl;
    }

    function showTooltip(e, text) {
        const tip = getTooltip();
        tip.textContent = text;
        tip.style.display = 'block';
        tip.style.left = e.pageX + 'px';
        tip.style.top = e.pageY + 'px';
    }

    function hideTooltip() {
        const tip = getTooltip();
        tip.style.display = 'none';
    }

    function renderHeatmap(data, engKey = 'avg_likes') {
        const container = document.getElementById('heatmap-container');
        container.replaceChildren();
        if (!data || data.length === 0) { showEmptyState(container); return; }

        const mainContainer = document.createElement('div');
        mainContainer.className = 'calendar-heatmap-container';
        mainContainer.style.display = 'flex';
        mainContainer.style.flexDirection = 'column';
        mainContainer.style.gap = '5px';

        const headerRow = document.createElement('div');
        headerRow.style.display = 'flex';
        headerRow.style.marginLeft = '40px';

        for (let h = 0; h < 24; h++) {
            const el = document.createElement('div');
            el.className = 'heatmap-label';
            el.style.flex = '1';
            el.style.textAlign = 'center';
            el.textContent = h;
            headerRow.appendChild(el);
        }
        mainContainer.appendChild(headerRow);

        const gridBody = document.createElement('div');
        gridBody.style.display = 'flex';
        gridBody.style.flexDirection = 'column';
        gridBody.style.gap = '2px';

        const days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
        const maxVal = Math.max(...data.map(d => d[engKey]));
        const dataMap = {};
        data.forEach(d => { dataMap[`${d.day_of_week}-${d.hour_of_day}`] = d[engKey]; });
        const tooltipLabel = engKey === 'avg_views' ? 'Avg Views' : 'Avg Likes';

        for (let d = 0; d < 7; d++) {
            const row = document.createElement('div');
            row.style.display = 'flex';
            row.style.alignItems = 'center';
            row.style.gap = '2px';

            const label = document.createElement('div');
            label.className = 'heatmap-label';
            label.style.width = '40px';
            label.textContent = days[d];
            row.appendChild(label);

            for (let h = 0; h < 24; h++) {
                const val = dataMap[`${d}-${h}`] || 0;
                const cell = document.createElement('div');
                cell.className = 'heatmap-cell';
                cell.style.flex = '1';
                cell.style.aspectRatio = '1';
                cell.style.backgroundColor = 'rgba(255,255,255,0.02)';
                cell.style.borderRadius = '2px';

                if (val > 0) {
                    const intensity = maxVal > 0 ? (val / maxVal) : 0;
                    cell.style.backgroundColor = `rgba(145, 103, 228, ${intensity * 0.9 + 0.1})`;
                }

                cell.addEventListener('mousemove', (e) => showTooltip(e, `${days[d]} ${h}:00 - ${tooltipLabel}: ${Math.round(val)}`));
                cell.addEventListener('mouseleave', hideTooltip);

                row.appendChild(cell);
            }
            gridBody.appendChild(row);
        }
        mainContainer.appendChild(gridBody);
        container.appendChild(mainContainer);
    }

    function loadPostingConsistency() {
        fetch(getFilteredUrl('/analytics/data/consistency'))
            .then(res => res.json())
            .then(data => {
                renderCalendarHeatmap(data || []);
            });
    }

    function getWeekNumber(d) {
        d = new Date(Date.UTC(d.getFullYear(), d.getMonth(), d.getDate()));
        d.setUTCDate(d.getUTCDate() + 4 - (d.getUTCDay() || 7));
        var yearStart = new Date(Date.UTC(d.getUTCFullYear(), 0, 1));
        var weekNo = Math.ceil((((d - yearStart) / 86400000) + 1) / 7);
        return weekNo;
    }

    function renderCalendarHeatmap(data) {
        const container = document.getElementById('calendar-heatmap-container');
        container.replaceChildren();
        if (!data || data.length === 0) { showEmptyState(container); return; }

        const weeklyData = Array.from({ length: 53 }, () => Array(7).fill(0));

        data.forEach(d => {
            const date = new Date(d.date_str);
            const weekNum = getWeekNumber(date);
            const dayOfWeek = date.getDay();

            if (weekNum >= 1 && weekNum <= 52) {
                weeklyData[weekNum][dayOfWeek] += d.post_count;
            }
        });

        const mainContainer = document.createElement('div');
        mainContainer.className = 'calendar-heatmap-container';
        mainContainer.style.display = 'flex';
        mainContainer.style.flexDirection = 'column';
        mainContainer.style.gap = '5px';
        mainContainer.style.width = '100%';

        const monthRow = document.createElement('div');
        monthRow.style.display = 'flex';
        monthRow.style.position = 'relative';
        monthRow.style.height = '20px';
        monthRow.style.width = '100%';
        monthRow.style.paddingLeft = '40px';
        monthRow.style.overflow = 'hidden';
        monthRow.style.paddingRight = '20px';

        const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
        months.forEach((m, i) => {
            const label = document.createElement('div');
            label.className = 'heatmap-label';
            label.textContent = m;
            label.style.position = 'absolute';
            label.style.left = `calc(${(i / 12) * 100}% + 40px)`;
            label.style.whiteSpace = 'nowrap';
            monthRow.appendChild(label);
        });
        mainContainer.appendChild(monthRow);

        const gridBody = document.createElement('div');
        gridBody.style.display = 'flex';
        gridBody.style.flexDirection = 'column';
        gridBody.style.gap = '2px';

        const days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];

        let maxVal = 1;
        for (let w = 1; w <= 52; w++) {
            for (let d = 0; d < 7; d++) {
                maxVal = Math.max(maxVal, weeklyData[w][d]);
            }
        }

        for (let d = 0; d < 7; d++) {
            const row = document.createElement('div');
            row.style.display = 'flex';
            row.style.alignItems = 'center';
            row.style.gap = '2px';

            const label = document.createElement('div');
            label.className = 'heatmap-label';
            label.style.width = '40px';
            label.textContent = days[d];
            row.appendChild(label);

            for (let w = 1; w <= 52; w++) {
                const count = weeklyData[w][d];
                const cell = document.createElement('div');
                cell.className = 'heatmap-cell';
                cell.style.flex = '1';
                cell.style.aspectRatio = '1';
                cell.style.backgroundColor = 'rgba(255,255,255,0.02)';
                cell.style.borderRadius = '2px';

                if (count > 0) {
                    const intensity = count / maxVal;
                    cell.style.backgroundColor = `rgba(145, 103, 228, ${intensity * 0.8 + 0.2})`;
                }

                cell.addEventListener('mousemove', (e) => showTooltip(e, `Week ${w}, ${days[d]}: ${count} posts`));
                cell.addEventListener('mouseleave', hideTooltip);

                row.appendChild(cell);
            }
            gridBody.appendChild(row);
        }
        mainContainer.appendChild(gridBody);
        container.appendChild(mainContainer);
    }

    function loadWebsiteStats() {
        Promise.all([
            fetch(getFilteredUrl('/analytics/data/site')).then(r => r.json()),
            fetch(getFilteredUrl('/analytics/data/pages')).then(r => r.json()),
            fetch(getFilteredUrl('/analytics/data/gsc/site')).then(r => r.json()),
            fetch(getFilteredUrl('/analytics/data/gsc/pages')).then(r => r.json())
        ]).then(([siteData, pagesData, gscSiteData, gscPagesData]) => {
            if (!siteData || !Array.isArray(siteData) || siteData.length === 0) {
                emptyChartState('siteStatsChart');
            } else {
                createChart('siteStatsChart', 'line', {
                    labels: siteData.map(d => d.date_str),
                    datasets: [{
                        label: 'Visitors',
                        data: siteData.map(d => d.total_visitors),
                        borderColor: colors.primary,
                        backgroundColor: 'rgba(145, 103, 228, 0.1)',
                        fill: true,
                        yAxisID: 'y'
                    }, {
                        label: 'Avg Session (s)',
                        data: siteData.map(d => d.avg_session_duration),
                        borderColor: colors.primaryLight,
                        borderDash: [5, 5],
                        yAxisID: 'y1'
                    }]
                }, {
                    scales: {
                        y: { position: 'left', title: { display: true, text: 'Visitors' } },
                        y1: { position: 'right', title: { display: true, text: 'Seconds' }, grid: { drawOnChartArea: false } }
                    },
                    plugins: {
                        legend: { labels: { usePointStyle: true, padding: 20 } }
                    }
                });
            }
            if (!pagesData || !Array.isArray(pagesData) || pagesData.length === 0) {
                emptyChartState('topPagesChart');
            } else {
                createChart('topPagesChart', 'bar', {
                    labels: pagesData.slice(0, 15).map(d => d.url_path),
                    datasets: [{
                        label: 'Total Views',
                        data: pagesData.slice(0, 15).map(d => d.total_views),
                        backgroundColor: colors.primary
                    }]
                }, {
                    indexAxis: 'y',
                    plugins: {
                        legend: { labels: { usePointStyle: true, padding: 20 } }
                    }
                });
            }

            if (!gscSiteData || !Array.isArray(gscSiteData) || gscSiteData.length === 0) {
                emptyChartState('gscSiteChart');
            } else {
                createChart('gscSiteChart', 'line', {
                    labels: gscSiteData.map(d => d.date_str),
                    datasets: [{
                        label: 'Clicks',
                        data: gscSiteData.map(d => d.total_clicks),
                        borderColor: colors.primary,
                        backgroundColor: 'rgba(145, 103, 228, 0.1)',
                        fill: true,
                        yAxisID: 'y'
                    }, {
                        label: 'Impressions',
                        data: gscSiteData.map(d => d.total_impressions),
                        borderColor: colors.primaryLight,
                        borderDash: [5, 5],
                        yAxisID: 'y1'
                    }]
                }, {
                    scales: {
                        y: { position: 'left', title: { display: true, text: 'Clicks' } },
                        y1: { position: 'right', title: { display: true, text: 'Impressions' }, grid: { drawOnChartArea: false } }
                    },
                    plugins: {
                        legend: { labels: { usePointStyle: true, padding: 20 } }
                    }
                });
            }

            if (!gscPagesData || !Array.isArray(gscPagesData) || gscPagesData.length === 0) {
                emptyChartState('gscPagesChart');
            } else {
                createChart('gscPagesChart', 'bar', {
                    labels: gscPagesData.slice(0, 15).map(d => d.url_path),
                    datasets: [{
                        label: 'Clicks',
                        data: gscPagesData.slice(0, 15).map(d => d.total_clicks),
                        backgroundColor: colors.primary
                    }, {
                        label: 'Impressions',
                        data: gscPagesData.slice(0, 15).map(d => d.total_impressions),
                        backgroundColor: colors.primaryLight
                    }]
                }, {
                    indexAxis: 'y',
                    plugins: {
                        legend: { labels: { usePointStyle: true, padding: 20 } }
                    }
                });
            }
        });
    }


    function loadEngagementRate() {
        fetch(getFilteredUrl('/analytics/data/engagement-rate'))
            .then(res => res.json())
            .then(data => {
                if (!data || !Array.isArray(data)) return;

                const networks = [...new Set(data.map(d => d.network))];
                const aggregatedData = networks.map((network, i) => {
                    const netData = data.filter(d => d.network === network);
                    if (netData.length === 0) return null;

                    const totalEngagement = isViewsMode()
                        ? netData.reduce((sum, d) => sum + d.views, 0)
                        : netData.reduce((sum, d) => sum + d.likes + d.reposts, 0);
                    const maxFollowers = Math.max(...netData.map(d => d.followers_count));

                    if (maxFollowers === 0) return null;

                    const avgRate = (totalEngagement / maxFollowers) * 100;

                    return {
                        x: network,
                        y: avgRate,
                        r: 10
                    };
                }).filter(d => d !== null);

                if (aggregatedData.length === 0) { emptyChartState('engagementRateChart'); return; }
                createChart('engagementRateChart', 'bubble', {
                    datasets: [{
                        label: 'Avg Engagement Rate',
                        data: aggregatedData,
                        backgroundColor: networks.map((_, i) => colors.highContrast[i % colors.highContrast.length]),
                        borderColor: networks.map((_, i) => colors.highContrast[i % colors.highContrast.length]),
                    }]
                }, {
                    scales: {
                        x: {
                            type: 'category',
                            labels: networks,
                            title: { display: true, text: 'Network' },
                            offset: true
                        },
                        y: {
                            title: { display: true, text: 'Avg Engagement Rate (%)' },
                            beginAtZero: true
                        }
                    },
                    plugins: {
                        legend: {
                            labels: {
                                usePointStyle: true,
                                padding: 20
                            }
                        },
                        tooltip: {
                            callbacks: {
                                label: (ctx) => `${ctx.raw.x}: ${ctx.raw.y.toFixed(3)}%`
                            }
                        }
                    }
                });
            });
    }

    function loadFollowRatio() {
        fetch('/analytics/data/follow-ratio')
            .then(res => res.json())
            .then(data => {
                if (!data || !Array.isArray(data) || data.length === 0) { emptyChartState('followRatioChart'); return; }

                const labels = data.map(d => d.network);
                const ratios = data.map(d => d.following_count > 0 ? d.followers_count / d.following_count : 0);
                const targetLine = new Array(data.length).fill(0.5);

                createChart('followRatioChart', 'bar', {
                    labels: labels,
                    datasets: [{
                        label: 'Follow Ratio (Following/Followers)',
                        data: ratios,
                        backgroundColor: colors.primary,
                        order: 2
                    }, {
                        label: 'Target',
                        data: targetLine,
                        type: 'line',
                        borderColor: 'rgba(255, 99, 132, 0.5)',
                        pointRadius: 0,
                        fill: false,
                        order: 1
                    }]
                }, {
                    scales: {
                        y: {
                            beginAtZero: true,
                            title: { display: true, text: 'Ratio' }
                        }
                    },
                    plugins: {
                        legend: {
                            labels: {
                                usePointStyle: true,
                                padding: 20
                            }
                        }
                    }
                });
            });
    }

    function loadCollaborations() {
        fetch(getFilteredUrl('/analytics/data/collaborations'))
            .then(res => res.json())
            .then(data => {
                if (!data || !Array.isArray(data) || data.length === 0) { emptyChartState('collaborationsChart'); return; }
                const engKey = isViewsMode() ? 'avg_views' : 'avg_likes';
                const engLbl = engagementLabel();
                const sorted = [...data].sort((a, b) => b[engKey] - a[engKey]);
                createChart('collaborationsChart', 'bar', {
                    labels: sorted.map(d => d.collaborator),
                    datasets: [{
                        label: engLbl,
                        data: sorted.map(d => d[engKey]),
                        backgroundColor: colors.primary,
                        yAxisID: 'y',
                        order: 2
                    }, {
                        label: 'Collaboration Count',
                        data: sorted.map(d => d.collaboration_count),
                        type: 'line',
                        borderColor: colors.accent,
                        backgroundColor: colors.accent,
                        yAxisID: 'y1',
                        order: 1
                    }]
                }, {
                    scales: {
                        y: { title: { display: true, text: engLbl } },
                        y1: { position: 'right', title: { display: true, text: 'Count' }, grid: { drawOnChartArea: false } }
                    },
                    plugins: {
                        legend: {
                            labels: {
                                usePointStyle: true,
                                padding: 20
                            }
                        }
                    }
                });
            });
    }

    function loadTopInteracted() {
        const url = getFilteredUrl('/analytics/data/top-interacted');
        fetch(url)
            .then(res => res.json())
            .then(data => {
                if (!data) return;

                const renderTable = (items, tableId) => {
                    const tbody = document.querySelector(`#${tableId} tbody`);
                    if (!tbody) return;
                    tbody.replaceChildren();

                    if (!items || items.length === 0) {
                        const tr = document.createElement('tr');
                        const td = document.createElement('td');
                        td.colSpan = 5;
                        td.className = 'p-6 text-center text-muted opacity-50';
                        td.textContent = 'Not enough data';
                        tr.appendChild(td);
                        tbody.appendChild(tr);
                        return;
                    }

                    items.forEach(d => {
                        const row = document.createElement('tr');
                        row.className = 'hover:bg-white/5 transition-colors';

                        const dateCell = document.createElement('td');
                        dateCell.className = 'p-2 border-b border-white/10 text-sm text-gray-400';
                        dateCell.textContent = new Date(d.created_at).toLocaleDateString();
                        row.appendChild(dateCell);

                        const networkCell = document.createElement('td');
                        networkCell.className = 'p-2 border-b border-white/10';
                        const badge = document.createElement('span');
                        badge.className = 'badge badge-outline';
                        badge.textContent = d.network;
                        networkCell.appendChild(badge);
                        row.appendChild(networkCell);

                        const contentCell = document.createElement('td');
                        contentCell.className = 'p-2 border-b border-white/10 text-sm truncate max-w-xs';
                        contentCell.title = d.content || '';
                        if (d.url) {
                            const link = document.createElement('a');
                            link.href = d.url;
                            link.target = '_blank';
                            link.rel = 'noopener noreferrer';
                            link.className = 'text-accent hover:underline';
                            link.textContent = d.content ? d.content.substring(0, 50) + '...' : 'View Post';
                            contentCell.appendChild(link);
                        } else {
                            contentCell.textContent = d.content ? d.content.substring(0, 50) + '...' : 'Media';
                        }
                        row.appendChild(contentCell);

                        const deltaCell = document.createElement('td');
                        deltaCell.className = 'p-2 border-b border-white/10 text-sm font-bold text-success';
                        deltaCell.textContent = `+${d.engagement_delta}`;
                        row.appendChild(deltaCell);

                        const totalCell = document.createElement('td');
                        totalCell.className = 'p-2 border-b border-white/10 text-sm';
                        totalCell.textContent = d.engagement_total;
                        row.appendChild(totalCell);

                        tbody.appendChild(row);
                    });
                };

                renderTable(data.since_1day, 'topInteracted1DayTable');
                renderTable(data.since_14days, 'topInteracted14DaysTable');
            });
    }

    function loadPerformanceDeviation() {
        fetch(getFilteredUrl('/analytics/data/performance-deviation'))
            .then(res => res.json())
            .then(data => {
                if (!data) return;

                const renderTable = (items, tableId) => {
                    const tbody = document.querySelector(`#${tableId} tbody`);
                    tbody.replaceChildren();

                    if (!items || items.length === 0) {
                        const tr = document.createElement('tr');
                        const td = document.createElement('td');
                        td.colSpan = 5;
                        td.className = 'p-6 text-center text-muted opacity-50';
                        td.textContent = 'Not enough data';
                        tr.appendChild(td);
                        tbody.appendChild(tr);
                        return;
                    }

                    items.forEach(d => {
                        const row = document.createElement('tr');
                        row.className = 'hover:bg-white/5 transition-colors';

                        const date = new Date(d.created_at).toLocaleDateString();
                        const engagement = isViewsMode() ? d.views : d.likes + d.reposts;
                        const baseline = d.expected_engagement;
                        const deviation = isViewsMode() ? d.views - d.expected_engagement : d.deviation;

                        const dateCell = document.createElement('td');
                        dateCell.className = 'p-2 border-b border-white/10 text-sm text-gray-400';
                        dateCell.textContent = date;
                        row.appendChild(dateCell);

                        const networkCell = document.createElement('td');
                        networkCell.className = 'p-2 border-b border-white/10';
                        const badge = document.createElement('span');
                        badge.className = 'badge badge-outline';
                        badge.textContent = d.network;
                        networkCell.appendChild(badge);
                        row.appendChild(networkCell);

                        const contentCell = document.createElement('td');
                        contentCell.className = 'p-2 border-b border-white/10 text-sm truncate max-w-xs';
                        contentCell.title = d.content || '';

                        if (d.url) {
                            const link = document.createElement('a');
                            link.href = d.url;
                            link.target = '_blank';
                            link.rel = 'noopener noreferrer';
                            link.className = 'text-accent hover:underline';
                            link.textContent = d.content ? d.content.substring(0, 50) + '...' : 'View Post';
                            contentCell.appendChild(link);
                        } else {
                            contentCell.textContent = d.content ? d.content.substring(0, 50) + '...' : 'Media';
                        }
                        row.appendChild(contentCell);

                        const engagementCell = document.createElement('td');
                        engagementCell.className = 'p-2 border-b border-white/10 text-sm';
                        engagementCell.textContent = `${engagement} (Exp: ${Math.round(baseline)})`;
                        row.appendChild(engagementCell);

                        const deviationCell = document.createElement('td');
                        const isPositive = deviation >= 0;

                        deviationCell.className = `p-2 border-b border-white/10 font-bold ${isPositive ? 'text-success' : 'text-soft-danger'}`;
                        deviationCell.textContent = `${isPositive ? '+' : ''}${Math.round(deviation)}`;
                        row.appendChild(deviationCell);

                        tbody.appendChild(row);
                    });
                };

                renderTable(data.positive, 'performanceDeviationPositiveTable');
                renderTable(data.negative, 'performanceDeviationNegativeTable');
            });
    }

    function loadEngagementVelocity() {
        fetch(getFilteredUrl('/analytics/data/velocity'))
            .then(res => res.json())
            .then(data => {
                if (!data || !Array.isArray(data) || data.length === 0) {
                    emptyChartState('velocityChart');
                    const tbody = document.querySelector('#velocityTable tbody');
                    if (tbody) {
                        tbody.replaceChildren();
                        const tr = document.createElement('tr');
                        const td = document.createElement('td');
                        td.colSpan = 5;
                        td.className = 'p-6 text-center text-muted opacity-50';
                        td.textContent = 'Not enough data';
                        tr.appendChild(td);
                        tbody.appendChild(tr);
                    }
                    return;
                }
                const posts = {};
                const postDetails = {};

                const viewsMode = isViewsMode();

                data.forEach(d => {
                    if (!posts[d.post_id]) {
                        posts[d.post_id] = {
                            created_at: new Date(d.post_created_at),
                            content: d.content,
                            history: []
                        };
                        postDetails[d.post_id] = {
                            date: new Date(d.post_created_at),
                            content: d.content,
                            likes: d.likes,
                            reposts: d.reposts,
                            views: d.views,
                            url: d.url
                        };
                    }

                    if (d.likes > postDetails[d.post_id].likes) postDetails[d.post_id].likes = d.likes;
                    if (d.reposts > postDetails[d.post_id].reposts) postDetails[d.post_id].reposts = d.reposts;
                    if (d.views > postDetails[d.post_id].views) postDetails[d.post_id].views = d.views;

                    posts[d.post_id].history.push({
                        t: new Date(d.history_synced_at),
                        engagement: viewsMode ? d.views : d.likes + d.reposts
                    });
                });

                const tbody = document.querySelector('#velocityTable tbody');
                tbody.replaceChildren();

                const velocityThead = document.querySelector('#velocityTable thead tr');
                if (velocityThead) {
                    const ths = velocityThead.querySelectorAll('th');
                    if (viewsMode) {
                        if (ths[2]) ths[2].textContent = 'Views';
                        if (ths[3]) ths[3].textContent = '';
                        if (ths[4]) ths[4].textContent = 'Total';
                    } else {
                        if (ths[2]) ths[2].textContent = 'Likes';
                        if (ths[3]) ths[3].textContent = 'Reposts';
                        if (ths[4]) ths[4].textContent = 'Total';
                    }
                }

                const sortedDetails = viewsMode
                    ? Object.values(postDetails).sort((a, b) => b.views - a.views).slice(0, 7)
                    : Object.values(postDetails).sort((a, b) => (b.likes + b.reposts) - (a.likes + a.reposts)).slice(0, 7);

                sortedDetails.forEach(p => {
                    const row = document.createElement('tr');
                    row.className = 'hover:bg-white/5 transition-colors';

                    const dateCell = document.createElement('td');
                    dateCell.className = 'p-2 border-b border-white/10 text-sm text-gray-400';
                    dateCell.textContent = p.date.toLocaleDateString();
                    row.appendChild(dateCell);

                    const contentCell = document.createElement('td');
                    contentCell.className = 'p-2 border-b border-white/10 text-sm truncate max-w-xs';
                    if (p.url) {
                        const link = document.createElement('a');
                        link.href = p.url;
                        link.target = '_blank';
                        link.rel = 'noopener noreferrer';
                        link.className = 'text-accent hover:underline';
                        link.textContent = p.content ? p.content.substring(0, 50) + '...' : 'View Post';
                        contentCell.appendChild(link);
                    } else {
                        contentCell.textContent = p.content ? p.content.substring(0, 50) + '...' : 'Media';
                    }
                    row.appendChild(contentCell);

                    const likesCell = document.createElement('td');
                    likesCell.className = 'p-2 border-b border-white/10 text-sm';
                    likesCell.textContent = viewsMode ? p.views : p.likes;
                    row.appendChild(likesCell);

                    const repostsCell = document.createElement('td');
                    repostsCell.className = 'p-2 border-b border-white/10 text-sm';
                    repostsCell.textContent = viewsMode ? '' : p.reposts;
                    row.appendChild(repostsCell);

                    const totalCell = document.createElement('td');
                    totalCell.className = 'p-2 border-b border-white/10 text-sm font-bold';
                    totalCell.textContent = viewsMode ? p.views : p.likes + p.reposts;
                    row.appendChild(totalCell);

                    tbody.appendChild(row);
                });

                const sortedPostIds = Object.keys(posts).sort((a, b) => {
                    const lastA = posts[a].history[posts[a].history.length - 1].engagement;
                    const lastB = posts[b].history[posts[b].history.length - 1].engagement;
                    return lastB - lastA;
                }).slice(0, 7);

                const datasets = sortedPostIds.map((pid, i) => {
                    const post = posts[pid];
                    const dataPoints = post.history.map(h => ({
                        x: (h.t.getTime() - post.created_at.getTime()) / 3600000,
                        y: h.engagement
                    })).filter(p => p.x >= 0);

                    return {
                        label: `${i + 1}. ${post.content ? post.content.substring(0, 30) + '...' : 'Media'}`,
                        data: dataPoints,
                        borderColor: colors.highContrast[i % colors.highContrast.length],
                        backgroundColor: 'transparent',
                        tension: 0.4
                    };
                });

                createChart('velocityChart', 'line', {
                    datasets: datasets
                }, {
                    scales: {
                        x: { type: 'linear', title: { display: true, text: 'Hours since posted' } },
                        y: { title: { display: true, text: viewsMode ? 'Total Views' : 'Total Engagement' } }
                    },
                    plugins: {
                        legend: {
                            labels: {
                                usePointStyle: true,
                                padding: 20
                            }
                        },
                        tooltip: {
                            callbacks: {
                                label: function (context) {
                                    return context.dataset.label + ': ' + context.parsed.y;
                                }
                            }
                        }
                    }
                });
            });
    }
    const ALL_POST_TYPES = ['post', 'image', 'video', 'broadcast', 'thread', 'album', 'quote', 'repost', 'tag', 'collab'];

    function updateDateBtnState() {
        const btn = document.getElementById('analyticsDateBtn');
        if (!btn) return;
        const active = !!(filterState.startDate || filterState.endDate);
        btn.classList.toggle('btn-primary', active);
        btn.classList.toggle('btn-secondary', !active);
    }

    function updatePostTypesBtnState() {
        const btn = document.getElementById('analyticsPostTypesBtn');
        if (!btn) return;
        const active = filterState.postTypes !== null && filterState.postTypes.length !== ALL_POST_TYPES.length;
        btn.classList.toggle('btn-primary', active);
        btn.classList.toggle('btn-secondary', !active);
    }

    function getDateRange(range) {
        const now = new Date();
        const pad = n => String(n).padStart(2, '0');
        const fmt = d => `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`;
        const today = fmt(now);
        switch (range) {
            case 'today': return { start: today, end: today };
            case 'yesterday': {
                const y = new Date(now); y.setDate(y.getDate() - 1);
                const yd = fmt(y); return { start: yd, end: yd };
            }
            case 'thisWeek': {
                const d = new Date(now); d.setDate(d.getDate() - d.getDay());
                return { start: fmt(d), end: today };
            }
            case 'thisMonth': return { start: `${now.getFullYear()}-${pad(now.getMonth() + 1)}-01`, end: today };
            case 'thisYear': return { start: `${now.getFullYear()}-01-01`, end: today };
            case 'last7': { const d = new Date(now); d.setDate(d.getDate() - 6); return { start: fmt(d), end: today }; }
            case 'last30': { const d = new Date(now); d.setDate(d.getDate() - 29); return { start: fmt(d), end: today }; }
            case 'last365': { const d = new Date(now); d.setDate(d.getDate() - 364); return { start: fmt(d), end: today }; }
            default: return { start: '', end: '' };
        }
    }

    const ptContainer = document.getElementById('analyticsPostTypesOptions');
    if (ptContainer) {
        ALL_POST_TYPES.forEach(pt => {
            const label = document.createElement('label');
            label.className = 'flex items-center gap-2 py-1 cursor-pointer';
            const checkbox = document.createElement('input');
            checkbox.type = 'checkbox';
            checkbox.className = 'analytics-pt-check';
            checkbox.value = pt;
            checkbox.checked = true;
            label.appendChild(checkbox);
            label.appendChild(document.createTextNode(' ' + pt));
            ptContainer.appendChild(label);
        });
    }

    const dateBtn = document.getElementById('analyticsDateBtn');
    const dateMenu = document.getElementById('analyticsDateMenu');
    const ptBtn = document.getElementById('analyticsPostTypesBtn');
    const ptMenu = document.getElementById('analyticsPostTypesMenu');
    const modeBtn = document.getElementById('analyticsModeBtn');
    const modeMenu = document.getElementById('analyticsModeMenu');
    const tagsBtn = document.getElementById('analyticsTagsBtn');
    const tagsMenu = document.getElementById('analyticsTagsMenu');

    function closeAllFilterDropdowns() {
        [dateMenu, ptMenu, modeMenu, tagsMenu].forEach(m => {
            if (!m) return;
            m.classList.remove('show');
            m.style.right = '';
            m.style.left = '';
        });
    }

    function closeOtherDropdowns(except) {
        [dateMenu, ptMenu, modeMenu, tagsMenu].forEach(m => {
            if (m && m !== except) {
                m.classList.remove('show');
                m.style.right = '';
                m.style.left = '';
            }
        });
    }

    function toggleFilterDropdown(menu) {
        if (!menu) return;
        if (menu.classList.contains('show')) {
            menu.classList.remove('show');
            menu.style.right = '';
            menu.style.left = '';
            return;
        }
        menu.style.right = '0';
        menu.style.left = 'auto';
        menu.classList.add('show');
        const rect = menu.getBoundingClientRect();
        if (rect.left < 8) {
            menu.style.right = 'auto';
            menu.style.left = '0';
        }
        if (rect.right > window.innerWidth - 8) {
            menu.style.left = 'auto';
            menu.style.right = '0';
        }
    }

    dateBtn && dateBtn.addEventListener('click', e => {
        e.stopPropagation();
        closeOtherDropdowns(dateMenu);
        toggleFilterDropdown(dateMenu);
    });

    ptBtn && ptBtn.addEventListener('click', e => {
        e.stopPropagation();
        closeOtherDropdowns(ptMenu);
        toggleFilterDropdown(ptMenu);
    });

    modeBtn && modeBtn.addEventListener('click', e => {
        e.stopPropagation();
        closeOtherDropdowns(modeMenu);
        toggleFilterDropdown(modeMenu);
    });

    tagsBtn && tagsBtn.addEventListener('click', e => {
        e.stopPropagation();
        closeOtherDropdowns(tagsMenu);
        toggleFilterDropdown(tagsMenu);
    });

    dateMenu && dateMenu.addEventListener('click', e => e.stopPropagation());
    ptMenu && ptMenu.addEventListener('click', e => e.stopPropagation());
    modeMenu && modeMenu.addEventListener('click', e => e.stopPropagation());
    tagsMenu && tagsMenu.addEventListener('click', e => e.stopPropagation());

    document.addEventListener('click', closeAllFilterDropdowns);

    document.querySelectorAll('.analytics-date-suggestion').forEach(btn => {
        btn.addEventListener('click', () => {
            const { start, end } = getDateRange(btn.dataset.range);
            document.getElementById('analyticsStartDate').value = start;
            document.getElementById('analyticsEndDate').value = end;
        });
    });

    document.getElementById('analyticsApplyDates') && document.getElementById('analyticsApplyDates').addEventListener('click', () => {
        filterState.startDate = document.getElementById('analyticsStartDate').value;
        filterState.endDate = document.getElementById('analyticsEndDate').value;
        closeAllFilterDropdowns();
        updateDateBtnState();
        applyGlobalFilters();
    });

    document.getElementById('analyticsClearDates') && document.getElementById('analyticsClearDates').addEventListener('click', () => {
        document.getElementById('analyticsStartDate').value = '';
        document.getElementById('analyticsEndDate').value = '';
        filterState.startDate = '';
        filterState.endDate = '';
        closeAllFilterDropdowns();
        updateDateBtnState();
        applyGlobalFilters();
    });

    document.getElementById('analyticsPostTypesSelectAll') && document.getElementById('analyticsPostTypesSelectAll').addEventListener('click', () => {
        document.querySelectorAll('.analytics-pt-check').forEach(cb => cb.checked = true);
    });

    document.getElementById('analyticsPostTypesClear') && document.getElementById('analyticsPostTypesClear').addEventListener('click', () => {
        document.querySelectorAll('.analytics-pt-check').forEach(cb => cb.checked = false);
    });

    document.getElementById('analyticsApplyPostTypes') && document.getElementById('analyticsApplyPostTypes').addEventListener('click', () => {
        const checked = [...document.querySelectorAll('.analytics-pt-check:checked')].map(cb => cb.value);
        filterState.postTypes = (checked.length === ALL_POST_TYPES.length) ? null : checked;
        closeAllFilterDropdowns();
        updatePostTypesBtnState();
        applyGlobalFilters();
    });

    function updateModeBtnState() {
        if (!modeBtn) return;
        const views = isViewsMode();
        modeBtn.classList.toggle('btn-primary', views);
        modeBtn.classList.toggle('btn-secondary', !views);
        const label = document.getElementById('analyticsModeLabel');
        if (label) label.textContent = `Mode: ${views ? 'Views' : 'Likes'}`;
    }

    document.getElementById('analyticsApplyMode') && document.getElementById('analyticsApplyMode').addEventListener('click', () => {
        const selected = document.querySelector('.analytics-mode-radio:checked');
        if (selected && selected.value !== filterState.mode) {
            filterState.mode = selected.value;
            updateModeBtnState();
            applyGlobalFilters();
        }
        closeAllFilterDropdowns();
    });

    let allTagsData = [];

    function initTagsFilter() {
        fetch('/api/tags').then(r => r.json()).then(tags => {
            allTagsData = tags || [];
            buildTagFilterOptions();
        }).catch(err => console.error('Error loading tags for filter:', err));
    }

    function buildTagFilterOptions() {
        const container = document.getElementById('analyticsTagsOptions');
        if (!container) return;
        container.innerHTML = '';

        if (allTagsData.length === 0) {
            const empty = document.createElement('div');
            empty.className = 'text-sm text-muted p-2 text-center';
            empty.textContent = 'No tags defined yet.';
            container.appendChild(empty);
            return;
        }

        const grouped = {};
        const ungrouped = [];
        allTagsData.forEach(t => {
            if (t.classification_name) {
                if (!grouped[t.classification_name]) grouped[t.classification_name] = [];
                grouped[t.classification_name].push(t);
            } else {
                ungrouped.push(t);
            }
        });

        Object.keys(grouped).sort().forEach(clName => {
            const groupDiv = document.createElement('div');
            groupDiv.className = 'tag-filter-group';
            const groupLabel = document.createElement('div');
            groupLabel.className = 'tag-filter-group-label';
            groupLabel.textContent = clName;
            groupDiv.appendChild(groupLabel);
            grouped[clName].forEach(t => appendTagCheckbox(groupDiv, t));
            container.appendChild(groupDiv);
        });

        if (ungrouped.length > 0) {
            const groupDiv = document.createElement('div');
            groupDiv.className = 'tag-filter-group';
            if (Object.keys(grouped).length > 0) {
                const label = document.createElement('div');
                label.className = 'tag-filter-group-label';
                label.textContent = 'Uncategorized';
                groupDiv.appendChild(label);
            }
            ungrouped.forEach(t => appendTagCheckbox(groupDiv, t));
            container.appendChild(groupDiv);
        }
    }

    function appendTagCheckbox(container, tag) {
        const label = document.createElement('label');
        label.className = 'filter-option';
        const cb = document.createElement('input');
        cb.type = 'checkbox';
        cb.className = 'analytics-tag-check';
        cb.value = tag.id;
        cb.checked = true;
        label.appendChild(cb);
        label.appendChild(document.createTextNode(' ' + tag.name));
        container.appendChild(label);
    }

    function updateTagsBtnState() {
        if (!tagsBtn) return;
        const active = filterState.tagIds !== null;
        tagsBtn.classList.toggle('btn-primary', active);
        tagsBtn.classList.toggle('btn-secondary', !active);
    }

    function resolveSelectedTagIds() {
        const checkedTags = [...document.querySelectorAll('.analytics-tag-check:checked')].map(cb => cb.value);
        const allTags = [...document.querySelectorAll('.analytics-tag-check')];
        return checkedTags.length === allTags.length ? null : checkedTags;
    }

    document.getElementById('analyticsTagsSelectAll') && document.getElementById('analyticsTagsSelectAll').addEventListener('click', () => {
        document.querySelectorAll('.analytics-tag-check').forEach(cb => cb.checked = true);
    });

    document.getElementById('analyticsTagsClear') && document.getElementById('analyticsTagsClear').addEventListener('click', () => {
        document.querySelectorAll('.analytics-tag-check').forEach(cb => cb.checked = false);
    });

    document.getElementById('analyticsApplyTags') && document.getElementById('analyticsApplyTags').addEventListener('click', () => {
        filterState.tagIds = resolveSelectedTagIds();
        closeAllFilterDropdowns();
        updateTagsBtnState();
        applyGlobalFilters();
    });

    initTagsFilter();

    function loadTagAnalytics() {
        const metricLabel = isViewsMode() ? 'Avg Views' : 'Avg Likes';

        fetch(getFilteredUrl('/analytics/data/tags'))
            .then(res => res.json())
            .then(data => {
                if (!data || !Array.isArray(data) || data.length === 0) {
                    emptyChartState('tagPerformanceChart');
                    emptyChartState('tagPostCountChart');
                    const emptyTbody = document.querySelector('#tagAnalyticsTable tbody');
                    emptyTbody.innerHTML = '';
                    const emptyRow = document.createElement('tr');
                    const emptyCell = document.createElement('td');
                    emptyCell.colSpan = 8;
                    emptyCell.className = 'p-4 text-center text-muted';
                    emptyCell.textContent = 'No tag data available. Assign tags to posts to see analytics.';
                    emptyRow.appendChild(emptyCell);
                    emptyTbody.appendChild(emptyRow);
                    return;
                }

                const metricKey = isViewsMode() ? 'avg_views' : 'avg_likes';
                const tagColors = data.map((_, i) => colors.highContrast[i % colors.highContrast.length]);

                createChart('tagPerformanceChart', 'bar', {
                    labels: data.map(d => d.tag_name),
                    datasets: [{
                        label: metricLabel,
                        data: data.map(d => d[metricKey]),
                        backgroundColor: tagColors
                    }]
                }, {
                    indexAxis: 'y',
                    plugins: { legend: { display: false } }
                });

                const tbody = document.querySelector('#tagAnalyticsTable tbody');
                tbody.innerHTML = '';
                const cellClass = 'p-2 border-b border-white/5';
                data.forEach(d => {
                    const tr = document.createElement('tr');
                    const values = [
                        d.tag_name + ' / ' + (d.classification_name && d.classification_name.Valid ? d.classification_name.String : '-'), d.post_count, d.avg_likes + d.avg_reposts, d.avg_views
                    ];
                    values.forEach(v => {
                        const td = document.createElement('td');
                        td.className = cellClass;
                        td.textContent = v;
                        tr.appendChild(td);
                    });
                    tbody.appendChild(tr);
                });

                createChart('tagPostCountChart', 'doughnut', {
                    labels: data.map(d => d.tag_name),
                    datasets: [{
                        data: data.map(d => d.post_count),
                        backgroundColor: tagColors
                    }]
                }, {
                    plugins: { legend: { display: false } }
                });
            })
            .catch(err => console.error('Error loading tag analytics:', err));

        fetch(getFilteredUrl('/analytics/data/tags/classifications'))
            .then(res => res.json())
            .then(data => {
                if (!data || !Array.isArray(data) || data.length === 0) {
                    emptyChartState('classificationChart');
                    emptyChartState('classificationPostCountChart');
                    return;
                }

                const classMetricKey = isViewsMode() ? 'avg_views' : 'avg_likes';
                const classMetricLabel = isViewsMode() ? 'Avg Views' : 'Avg Likes';
                const classColors = data.map((_, i) => colors.highContrast[i % colors.highContrast.length]);

                createChart('classificationChart', 'bar', {
                    labels: data.map(d => d.classification_name),
                    datasets: [{
                        label: classMetricLabel,
                        data: data.map(d => d[classMetricKey]),
                        backgroundColor: classColors
                    }]
                }, {
                    indexAxis: 'y',
                    plugins: { legend: { display: false } }
                });

                createChart('classificationPostCountChart', 'doughnut', {
                    labels: data.map(d => d.classification_name),
                    datasets: [{
                        data: data.map(d => d.post_count),
                        backgroundColor: classColors
                    }]
                }, {
                    plugins: { legend: { display: false } }
                });
            })
            .catch(err => console.error('Error loading classification analytics:', err));
    }

});
