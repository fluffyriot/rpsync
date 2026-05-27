let postTagsMap = {};
let allUserTags = [];
let tagFilterActive = false;
let selectedTagIds = new Set();
let filterNoTags = false;

async function loadPostTags() {
  try {
    const [tagsRes, bulkRes] = await Promise.all([
      fetch('/api/tags'),
      fetch('/api/posts/tags/bulk')
    ]);
    if (tagsRes.ok) allUserTags = await tagsRes.json();
    if (bulkRes.ok) {
      const bulk = await bulkRes.json();
      postTagsMap = {};
      bulk.forEach(r => {
        if (!postTagsMap[r.post_id]) postTagsMap[r.post_id] = [];
        postTagsMap[r.post_id].push(r);
      });
    }
    renderTagBadgesInTable();
    if (!$('#tagFilterOptions').length) buildInlineTagFilter();
    buildTagFilterOptions();
  } catch (e) {
    console.error('Error loading tags:', e);
  }
}

function renderTagBadgesInTable() {
  $('#postsTable tbody tr').each(function () {
    const postId = $(this).data('post-id');
    const tags = postTagsMap[postId] || [];
    const $cell = $(this).find('td.tags-cell');
    if (tags.length > 0) {
      $cell.html(tags.map(t => '<span class="tag-badge">' + $('<span/>').text(t.tag_name).html() + '</span>').join(' '));
    } else {
      $cell.html('<span class="text-muted text-xs">-</span>');
    }
    $(this).attr('data-tag-ids', tags.map(t => t.tag_id).join(','));
  });
}

function buildTagFilterOptions() {
  const $container = $('#tagFilterOptions');
  if (!$container.length) return;
  $container.empty();

  const grouped = {};
  const ungrouped = [];
  allUserTags.forEach(t => {
    if (t.classification_name) {
      if (!grouped[t.classification_name]) grouped[t.classification_name] = [];
      grouped[t.classification_name].push(t);
    } else {
      ungrouped.push(t);
    }
  });

  Object.keys(grouped).sort().forEach(clName => {
    const $group = $('<div/>').addClass('tag-filter-group');
    $group.append($('<div/>').addClass('tag-filter-group-label').text(clName));
    grouped[clName].forEach(t => {
      const $label = $('<label/>').addClass('filter-option');
      $label.append($('<input/>').attr('type', 'checkbox').val(t.id).prop('checked', true));
      $label.append(document.createTextNode(' ' + t.name));
      $group.append($label);
    });
    $container.append($group);
  });

  if (ungrouped.length > 0) {
    const $group = $('<div/>').addClass('tag-filter-group');
    if (Object.keys(grouped).length > 0) {
      $group.append($('<div/>').addClass('tag-filter-group-label').text('Uncategorized'));
    }
    ungrouped.forEach(t => {
      const $label = $('<label/>').addClass('filter-option');
      $label.append($('<input/>').attr('type', 'checkbox').val(t.id).prop('checked', true));
      $label.append(document.createTextNode(' ' + t.name));
      $group.append($label);
    });
    $container.append($group);
  }
}

function buildInlineTagFilter() {
  const table = $('#postsTable').DataTable();
  const column = table.column(6);
  const header = $(column.header());
  const title = header.text();

  const headerContent = $('<div class="filter-header-content"></div>');
  const triggerIcon = $('<span class="filter-trigger"><i data-lucide="filter"></i></span>');
  const titleSpan = $('<span></span>').text(title);

  headerContent.append(triggerIcon).append(titleSpan);
  header.empty().append(headerContent);

  const dropdown = $('<div class="filter-dropdown tag-filter-dropdown"></div>');
  const actionRow = $('<div class="filter-actions"></div>');
  const selectAllBtn = $('<span class="filter-action-btn" id="tagSelectAll">Select All</span>');
  const clearBtn = $('<span class="filter-action-btn" id="tagSelectNone">Clear</span>');
  actionRow.append(selectAllBtn).append(clearBtn);
  dropdown.append(actionRow);

  dropdown.append($('<div id="tagFilterOptions"></div>'));

  dropdown.append($('<div class="border-t pt-2 mt-2"><label class="filter-option"><input type="checkbox" id="tagFilterNoTags"> No Tags</label></div>'));

  const buttonRow = $('<div class="flex justify-between pt-2 mt-2 border-t"></div>');
  buttonRow.append($('<button class="btn btn-sm btn-ghost" id="clearTagFilter">Reset</button>'));
  buttonRow.append($('<button class="btn btn-sm btn-primary" id="applyTagFilter">Apply</button>'));
  dropdown.append(buttonRow);

  header.find('.filter-header-content').append(dropdown);

  const trigger = header.find('.filter-trigger');

  trigger.on('click', function (e) {
    e.stopPropagation();
    $('.filter-dropdown').not(dropdown).removeClass('show');
    $('.filter-trigger').not(trigger).removeClass('active');

    if (dropdown.hasClass('show')) {
      dropdown.removeClass('show');
      trigger.removeClass('active');
    } else {
      const rect = this.getBoundingClientRect();
      const dropdownMaxWidth = 300;
      const fitsRight = rect.left + dropdownMaxWidth <= window.innerWidth;
      dropdown.css({
        'position': 'fixed',
        'top': (rect.bottom + 5) + 'px',
        'left': fitsRight ? rect.left + 'px' : 'auto',
        'right': fitsRight ? 'auto' : (window.innerWidth - rect.right) + 'px',
        'width': 'auto',
        'min-width': '200px',
        'max-width': '300px',
        'z-index': 10001
      });
      dropdown.addClass('show');
      trigger.addClass('active');
    }
  });

  dropdown.on('click', function (e) {
    e.stopPropagation();
  });

  $(document).on('click', function (e) {
    if (!$(e.target).closest('.filter-header-content, .filter-dropdown').length) {
      dropdown.removeClass('show');
      trigger.removeClass('active');
    }
  });

  $('#applyTagFilter').on('click', function () {
    const checked = [];
    $('#tagFilterOptions input:checked').each(function () {
      checked.push($(this).val());
    });
    const allChecked = $('#tagFilterOptions input').length === checked.length;
    filterNoTags = $('#tagFilterNoTags').is(':checked');

    if (allChecked && !filterNoTags) {
      tagFilterActive = false;
      trigger.removeClass('has-filter');
    } else {
      tagFilterActive = true;
      selectedTagIds = new Set(checked);
      trigger.addClass('has-filter');
    }
    table.draw();
    dropdown.removeClass('show');
    trigger.removeClass('active');
  });

  $('#clearTagFilter').on('click', function () {
    $('#tagFilterOptions input').prop('checked', true);
    $('#tagFilterNoTags').prop('checked', false);
    tagFilterActive = false;
    filterNoTags = false;
    selectedTagIds.clear();
    trigger.removeClass('has-filter');
    table.draw();
    dropdown.removeClass('show');
    trigger.removeClass('active');
  });

  $('#tagSelectAll').on('click', function () {
    $('#tagFilterOptions input').prop('checked', true);
  });

  $('#tagSelectNone').on('click', function () {
    $('#tagFilterOptions input').prop('checked', false);
  });

  lucide.createIcons();
}

async function addTagToPost(postId, tagId, tr) {
  const currentTags = postTagsMap[postId] || [];
  if (currentTags.length >= 5) {
    alert('A post cannot have more than 5 tags');
    return;
  }
  try {
    const res = await fetch(`/api/posts/${postId}/tags`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ tag_id: tagId })
    });
    if (!res.ok) throw new Error('Failed to add tag');
    await loadPostTags();
    refreshChildRow(tr);
  } catch (e) {
    alert('Error: ' + e.message);
  }
}

async function removeTagFromPost(postId, tagId, tr) {
  try {
    const res = await fetch(`/api/posts/${postId}/tags/${tagId}`, { method: 'DELETE' });
    if (!res.ok) throw new Error('Failed to remove tag');
    await loadPostTags();
    refreshChildRow(tr);
  } catch (e) {
    alert('Error: ' + e.message);
  }
}

function refreshChildRow(tr) {
  const $tr = $(tr);
  const row = $('#postsTable').DataTable().row($tr);
  if (row.child.isShown()) {
    row.child(formatChildRow($tr)).show();
    setTimeout(() => lucide.createIcons(), 0);
  }
}

function formatChildRow(tr) {
  const networkId = tr.data('network-id');
  const postId = tr.data('post-id');
  const author = tr.data('author');
  const likes = tr.data('likes');
  const reposts = tr.data('reposts');
  const status = tr.data('status');
  const fullContent = tr.data('full-content');
  const url = tr.data('url');
  const sourceId = tr.data('source-id');

  const $div = $('<div/>').addClass('child-row-details');
  const $info = $('<div/>').addClass('mb-4');

  const createRow = (label, value) => {
    const $row = $('<div/>').addClass('child-row-details-row');
    $row.append($('<strong/>').text(label + ': '));
    $row.append($('<span/>').addClass('child-row-value').text(' ' + (value || '-')));
    return $row;
  };

  $info.append(createRow('Author', author));
  $info.append(createRow('Internal ID', networkId));
  $info.append(createRow('Likes', likes));
  $info.append(createRow('Reposts', reposts));

  const $statusRow = $('<div/>');
  $statusRow.append($('<strong/>').text('Status: '));
  const badgeClass = status === 'Archived' ? 'badge-warning' : 'badge-success';
  const $badge = $('<span/>').addClass('badge status-badge ' + badgeClass).text(status);
  $statusRow.append(document.createTextNode(' '));
  $statusRow.append($badge);
  $info.append($statusRow);

  $info.append(createRow('Full Content', fullContent));

  const $tagsRow = $('<div/>').addClass('child-row-details-row mt-2');
  $tagsRow.append($('<strong/>').text('Tags: '));
  const $tagsContainer = $('<span/>').addClass('child-row-tags');
  const postTags = postTagsMap[postId] || [];
  postTags.forEach(tag => {
    const $tagBadge = $('<span/>').addClass('tag-badge tag-badge-removable').text(tag.tag_name);
    $tagBadge.attr('title', 'Click to remove');
    $tagBadge.on('click', function () {
      removeTagFromPost(postId, tag.tag_id, tr);
    });
    $tagsContainer.append($tagBadge);
    $tagsContainer.append(document.createTextNode(' '));
  });
  if (postTags.length === 0) {
    $tagsContainer.append($('<span/>').addClass('text-muted text-xs').text('No tags'));
  }
  $tagsRow.append($tagsContainer);

  if (postTags.length >= 5) {
    $tagsRow.append($('<span/>').addClass('text-muted text-xs').css('margin-left', '0.5rem').text('(max 5 tags)'));
  } else {
    const $addTagWrap = $('<span/>').css('margin-left', '0.5rem');
    const $addTagSelect = $('<select/>').addClass('form-select').css({ 'display': 'inline-block', 'width': 'auto', 'padding': '0.2rem 0.5rem', 'font-size': '0.75rem' });
    $addTagSelect.append($('<option/>').val('').text('+ Add tag...'));
    allUserTags.forEach(tag => {
      const alreadyAssigned = postTags.some(pt => pt.tag_id === tag.id);
      if (!alreadyAssigned) {
        const label = tag.classification_name ? `${tag.classification_name} / ${tag.name}` : tag.name;
        $addTagSelect.append($('<option/>').val(tag.id).text(label));
      }
    });
    $addTagSelect.on('change', function () {
      const tagId = $(this).val();
      if (tagId) {
        addTagToPost(postId, tagId, tr);
      }
    });
    $addTagWrap.append($addTagSelect);
    $tagsRow.append($addTagWrap);
  }

  $info.append($tagsRow);
  $div.append($info);

  const $buttons = $('<div/>').addClass('flex gap-2');

  if (url) {
    const $link = $('<a/>', {
      href: url,
      target: '_blank',
      rel: 'noopener noreferrer',
      class: 'btn btn-secondary btn-sm',
      text: ' Go to Post'
    });
    $link.prepend($('<i/>', { 'data-lucide': 'external-link', class: 'icon-sm' }));
    $buttons.append($link);
  }

  const $excludeBtn = $('<button/>', {
    class: 'btn btn-danger btn-sm',
    text: ' Remove from Sync'
  });
  $excludeBtn.prepend($('<i/>', { 'data-lucide': 'file-x', class: 'icon-sm' }));
  $excludeBtn.on('click', function () {
    excludePost(sourceId, networkId);
  });
  $buttons.append($excludeBtn);

  $div.append($buttons);

  setTimeout(() => lucide.createIcons(), 0);

  return $div;
}

async function excludePost(sourceId, networkInternalId) {
  if (!confirm('Are you sure you want to exclude this post? It will be deleted and not synced again.')) {
    return;
  }

  const data = {
    source_id: sourceId,
    network_internal_id: String(networkInternalId)
  };

  try {
    const response = await fetch('/api/exclusions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data)
    });

    if (!response.ok) {
      const error = await response.json();
      throw new Error(error.error || 'Failed to add exclusion');
    }

    alert('Exclude added successfully! The post has been deleted.');
    window.location.reload();
  } catch (error) {
    alert('Error: ' + error.message);
  }
}

$(document).ready(function () {
  $.fn.dataTable.ext.search.push(
    function (settings, data, dataIndex) {
      const minStr = $('#startDate').val();
      const maxStr = $('#endDate').val();

      if (!minStr && !maxStr) return true;

      const createdAtUnix = parseFloat(settings.aoData[dataIndex]._aData[1]['@data-order']) || 0;

      let minTime = null;
      if (minStr) {
        const parts = minStr.split('-');
        minTime = new Date(parts[0], parts[1] - 1, parts[2]).getTime() / 1000;
      }

      let maxTime = null;
      if (maxStr) {
        const parts = maxStr.split('-');
        const d = new Date(parts[0], parts[1] - 1, parts[2]);
        d.setDate(d.getDate() + 1);
        maxTime = d.getTime() / 1000;
      }

      if (minTime && maxTime) {
        return createdAtUnix >= minTime && createdAtUnix < maxTime;
      }
      if (minTime) {
        return createdAtUnix >= minTime;
      }
      if (maxTime) {
        return createdAtUnix < maxTime;
      }
      return true;
    }
  );

  $.fn.dataTable.ext.search.push(
    function (settings, data, dataIndex) {
      if (!tagFilterActive) return true;

      const tr = settings.aoData[dataIndex].nTr;
      const tagIdsStr = $(tr).attr('data-tag-ids') || '';
      const rowTagIds = tagIdsStr ? tagIdsStr.split(',') : [];

      if (filterNoTags) {
        return rowTagIds.length === 0;
      }

      if (selectedTagIds.size === 0) return false;
      return rowTagIds.some(id => selectedTagIds.has(id));
    }
  );

  const table = $('#postsTable').DataTable({
    order: [[1, 'desc']],
    responsive: false,
    scrollY: '60vh',
    scrollCollapse: true,
    paging: false,
    layout: {
      topStart: null,
      topEnd: 'search',
      bottomStart: null,
      bottomEnd: null
    },
    initComplete: function () {
      const $searchInput = $('#postsTable_wrapper .dt-search input');
      $searchInput.addClass('header-search-input').attr('placeholder', 'Search posts...');

      $searchInput.appendTo('#tableSearchContainer');
      $('#postsTable_wrapper .dt-search').remove();
    },
    ordering: true,
    lengthMenu: [[10, 25, 50, 100, -1], [10, 25, 50, 100, "All"]],
    columnDefs: [
      {
        orderable: false,
        className: 'details-control',
        targets: 0
      },
      {
        orderable: false,
        targets: [2, 5, 6, 7, 8]
      }
    ],
    language: {
      search: "Search posts: ",
      infoEmpty: "No posts found",
    },
    footerCallback: function (row, data, start, end, display) {
      var api = this.api();

      var intVal = function (i) {
        return typeof i === 'string' ?
          i.replace(/[\$,]/g, '') * 1 :
          typeof i === 'number' ?
            i : 0;
      };

      totalLikes = api
        .column(3, { search: 'applied' })
        .data()
        .reduce(function (a, b) {
          if (b === '-') return intVal(a);
          return intVal(a) + intVal(b);
        }, 0);

      totalViews = api
        .column(4, { search: 'applied' })
        .data()
        .reduce(function (a, b) {
          if (b === '-') return intVal(a);
          return intVal(a) + intVal(b);
        }, 0);

      totalRows = api.rows({ search: 'applied' }).count();

      $(api.column(2).footer()).html(totalRows + " posts");
      $(api.column(3).footer()).html(totalLikes + " interactions");
      $(api.column(4).footer()).html(totalViews + " views");
    }
  });

  let isFiltering = false;

  function updateDateButtonState() {
    const hasFilter = $('#startDate').val() || $('#endDate').val();
    const $btn = $('#dateFilterBtn');
    if (hasFilter) {
      $btn.removeClass('btn-secondary').addClass('btn-primary');
    } else {
      $btn.removeClass('btn-primary').addClass('btn-secondary');
    }
  }

  $('#applyDates').on('click', function () {
    updateDateButtonState();
    table.draw();
    $('#dateFilterDropdown .filter-dropdown').removeClass('show');
  });

  $('#clearDates').on('click', function () {
    $('#startDate').val('');
    $('#endDate').val('');
    updateDateButtonState();
    table.draw();
    $('#dateFilterDropdown .filter-dropdown').removeClass('show');
    $('.date-suggestion').removeClass('btn-primary').addClass('btn-ghost');
  });

  $('#dateFilterBtn').on('click', function (e) {
    e.stopPropagation();
    $('#dateFilterDropdown .filter-dropdown').toggleClass('show');
  });

  $('#dateFilterDropdown .filter-dropdown').on('click', function (e) {
    e.stopPropagation();
  });

  $(document).on('click', function (e) {
    if (!$(e.target).closest('#dateFilterDropdown').length) {
      $('#dateFilterDropdown .filter-dropdown').removeClass('show');
    }
  });

  $('.date-suggestion').on('click', function () {
    const range = $(this).data('range');
    const today = new Date();
    let start = new Date();
    let end = new Date();

    switch (range) {
      case 'today':
        break;
      case 'yesterday':
        start.setDate(today.getDate() - 1);
        end.setDate(today.getDate() - 1);
        break;
      case 'thisWeek':
        const day = today.getDay();
        const diff = today.getDate() - day + (day == 0 ? -6 : 1);
        start.setDate(diff);
        break;
      case 'thisMonth':
        start = new Date(today.getFullYear(), today.getMonth(), 1);
        break;
      case 'thisYear':
        start = new Date(today.getFullYear(), 0, 1);
        break;
      case 'last7':
        start.setDate(today.getDate() - 6);
        break;
      case 'last30':
        start.setDate(today.getDate() - 29);
        break;
      case 'last365':
        start.setDate(today.getDate() - 364);
        break;
    }

    const formatDate = (date) => {
      const d = new Date(date);
      let month = '' + (d.getMonth() + 1);
      let day = '' + d.getDate();
      const year = d.getFullYear();

      if (month.length < 2) month = '0' + month;
      if (day.length < 2) day = '0' + day;

      return [year, month, day].join('-');
    }

    $('#startDate').val(formatDate(start));
    $('#endDate').val(formatDate(end));

    updateDateButtonState();
    table.draw();
    $('.date-suggestion').removeClass('btn-primary').addClass('btn-ghost');
    $(this).removeClass('btn-ghost').addClass('btn-primary');
  });

  $('#startDate, #endDate').on('input', function () {
    $('.date-suggestion').removeClass('btn-primary').addClass('btn-ghost');
  });

  $('#clearDates').on('click', function () {
    $('.date-suggestion').removeClass('btn-primary').addClass('btn-ghost');
  });

  table.columns([2, 5]).every(function () {
    const column = this;
    const header = $(column.header());
    const title = header.text();

    const headerContent = $('<div class="filter-header-content"></div>');
    const triggerIcon = $('<span class="filter-trigger"><i data-lucide="filter"></i></span>');
    const titleSpan = $('<span></span>').text(title);

    headerContent.append(triggerIcon).append(titleSpan);
    header.empty().append(headerContent);
    const dropdown = $('<div class="filter-dropdown"></div>');
    const actionRow = $('<div class="filter-actions"></div>');
    const selectAllBtn = $('<span class="filter-action-btn">Select All</span>');
    const clearBtn = $('<span class="filter-action-btn">Clear</span>');

    actionRow.append(selectAllBtn).append(clearBtn);
    dropdown.append(actionRow);

    const optionsContainer = $('<div></div>');
    dropdown.append(optionsContainer);

    const uniqueData = [];
    column.data().unique().sort().each(function (d) {
      if (!d) return;
      const parser = new DOMParser();
      const doc = parser.parseFromString(d, 'text/html');
      const val = doc.body.textContent.trim();
      if (val && val !== '-' && !uniqueData.includes(val)) {
        uniqueData.push(val);
      }
    });

    uniqueData.forEach(val => {
      const option = $('<label>').addClass('filter-option');
      const input = $('<input>').attr('type', 'checkbox').val(val).prop('checked', true);
      option.append(input).append(document.createTextNode(' ' + val));
      optionsContainer.append(option);
    });

    header.find('.filter-header-content').append(dropdown);
    lucide.createIcons();

    const trigger = header.find('.filter-trigger');

    trigger.on('click', function (e) {
      e.stopPropagation();
      $('.filter-dropdown').not(dropdown).removeClass('show');
      $('.filter-trigger').not(trigger).removeClass('active');

      if (dropdown.hasClass('show')) {
        dropdown.removeClass('show');
        trigger.removeClass('active');
      } else {
        const rect = this.getBoundingClientRect();
        const dropdownMaxWidth = 300;
        const fitsRight = rect.left + dropdownMaxWidth <= window.innerWidth;
        dropdown.css({
          'position': 'fixed',
          'top': (rect.bottom + 5) + 'px',
          'left': fitsRight ? rect.left + 'px' : 'auto',
          'right': fitsRight ? 'auto' : (window.innerWidth - rect.right) + 'px',
          'width': 'auto',
          'min-width': '200px',
          'max-width': '300px',
          'z-index': 10001
        });
        dropdown.addClass('show');
        trigger.addClass('active');
      }
    });
    dropdown.on('click', function (e) {
      e.stopPropagation();
    });

    function applyFilter() {
      const checked = [];
      optionsContainer.find('input:checked').each(function () {
        checked.push($(this).val());
      });

      const searchStr = checked.map(v => {
        return '^' + v.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '$';
      }).join('|');

      isFiltering = true;
      column.search(searchStr, true, false).draw();
      setTimeout(() => {
        isFiltering = false;
      }, 300);

      if (optionsContainer.find('input:not(:checked)').length > 0) {
        trigger.addClass('has-filter');
      } else {
        trigger.removeClass('has-filter');
      }
    }

    optionsContainer.on('change', 'input', applyFilter);

    selectAllBtn.on('click', function () {
      optionsContainer.find('input').prop('checked', true);
      applyFilter();
    });

    clearBtn.on('click', function () {
      optionsContainer.find('input').prop('checked', false);
      applyFilter();
    });

    $(document).on('click', function (e) {
      if (!$(e.target).closest('.filter-header-content, .filter-dropdown').length) {
        dropdown.removeClass('show');
        trigger.removeClass('active');
      }
    });
  });

  $('#postsTable tbody').on('click', 'td.details-control', function () {
    const tr = $(this).closest('tr');
    const row = table.row(tr);

    if (row.child.isShown()) {
      row.child.hide();
      tr.removeClass('shown');
    } else {
      row.child(formatChildRow(tr)).show();
      tr.addClass('shown');
    }
  });

  $('.dt-scroll-body, .dataTables_scrollBody').on('scroll', function () {
    if (isFiltering) return;
    $('.filter-dropdown.show').removeClass('show');
    $('.filter-trigger.active').removeClass('active');
  });

  loadPostTags();
});
