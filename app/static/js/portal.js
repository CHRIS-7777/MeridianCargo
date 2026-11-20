/*
 * Meridian Cargo customer portal - client behaviour
 * build 2024.11.3
 *
 * API base: /api/v2   (v1 was retired in March, do not use)
 * Auth for scripted access: POST /api/v2/auth/token -> Bearer JWT (HS256)
 * Ops notes: nightly report exports are mirrored to the file share, see ticket MC-904
 */
(function ($) {
  'use strict';
  var page = $('body').data('page');

  // ---------------------------------------------------------------- register
  if (page === 'register') {
    $('#register-form').on('submit', function (e) {
      e.preventDefault();
      var body = {
        company: $('#r-company').val(),
        full_name: $('#r-name').val(),
        email: $('#r-email').val(),
        username: $('#r-user').val(),
        password: $('#r-pass').val(),
        account_type: 'customer'
      };
      $.ajax({
        url: '/api/v2/register', method: 'POST', contentType: 'application/json',
        data: JSON.stringify(body)
      }).done(function () {
        $('#reg-msg').html('<div class="alert alert-success">Account created. <a href="/login">Sign in</a></div>');
        $('#register-form')[0].reset();
      }).fail(function (xhr) {
        var m = (xhr.responseJSON && xhr.responseJSON.error) || 'Registration failed';
        $('#reg-msg').empty().append($('<div class="alert alert-danger"></div>').text(m));
      });
    });
  }

  // ---------------------------------------------------------------- dashboard counter
  if (page === 'dashboard') {
    $('#count-btn').on('click', function () {
      var s = $('#count-status').val();
      $.getJSON('/api/v2/shipments/count', { status: s }).done(function (d) {
        $('#count-out').text(d.count + ' shipment(s) with status "' + s + '"');
      }).fail(function () { $('#count-out').text('Could not load count.'); });
    });
  }

  // ---------------------------------------------------------------- quote calculator
  if (page === 'quote') {
    var perKg = { Sea: 0.35, Air: 4.2, Road: 0.9 };
    function laneFactor(a, b) {
      var s = 0, i;
      for (i = 0; i < a.length; i++) { s += a.charCodeAt(i); }
      for (i = 0; i < b.length; i++) { s += b.charCodeAt(i) * 2; }
      return 0.8 + (s % 70) / 100;
    }
    var recalc = _.debounce(function () {
      var w = parseInt($('#q-weight').val(), 10) || 0;
      var m = $('#q-mode').val();
      var raw = Math.max(150, w * perKg[m] * laneFactor($('#q-origin').val(), $('#q-dest').val()));
      var cents = Math.round(_.round(raw, 2) * 100);
      $('#q-rate').val(cents);
      $('#q-total').text((cents / 100).toFixed(2));
    }, 120);
    $('#quote-form').on('input change', 'select, input', recalc);
    recalc();
  }

  // ---------------------------------------------------------------- documents
  if (page === 'documents') {
    $('#upload-form').on('submit', function (e) {
      var f = $('#doc-file').val().toLowerCase();
      if (f && !/\.(pdf|jpg|jpeg|png)$/.test(f)) {
        e.preventDefault();
        alert('Only PDF, JPG or PNG files are accepted.');
      }
    });
  }

  // ---------------------------------------------------------------- webhooks
  if (page === 'integrations') {
    $('#hook-test').on('click', function () {
      var out = $('#hook-out').text('Testing...');
      $.ajax({
        url: '/api/v2/webhooks/test', method: 'POST', contentType: 'application/json',
        data: JSON.stringify({ url: $('#hook-url').val() })
      }).done(function (d) { out.text(JSON.stringify(d, null, 2)); })
        .fail(function (x) { out.text('Request failed: ' + x.status); });
    });
  }
})(jQuery);
