(function() {
  const lang = (document.documentElement && document.documentElement.getAttribute('lang')) || document.querySelector('html')?.getAttribute('lang') || 'es';
  const isEn = (lang === 'en');
  function d(enVal, esVal) { return isEn ? enVal : (esVal || enVal); }
  const t = window.HostBerry && window.HostBerry.t ? function(k, fallback) { return HostBerry.t(k, fallback); } : function(_, fallback) { return fallback || _; };
  const showAlert = window.HostBerry && window.HostBerry.showAlert ? function(type, msg) { HostBerry.showAlert(type, msg); } : function(_, msg) { alert(msg); };
  const apiRequest = window.HostBerry && window.HostBerry.apiRequest ? function(u, o) { return HostBerry.apiRequest(u, o); } : function(u, o) { return fetch(u, Object.assign({ credentials: 'include' }, o)); };

  let selectedSSID = null;
  let currentConnectedSSID = null; // SSID al que está conectado el dispositivo (desde wifi/status)
  let wizardStep = 1;
  let selectedSecurityOption = null; // 'vpn' | 'wireguard' | 'tor'

  function setStep(step) {
    wizardStep = step;
    document.querySelectorAll('.setup-step').forEach(function(el) {
      el.classList.toggle('d-none', parseInt(el.getAttribute('data-step'), 10) !== step);
    });
    document.querySelectorAll('.step-dot').forEach(function(el) {
      el.classList.toggle('active', parseInt(el.getAttribute('data-step'), 10) === step);
    });
  }

  function signalBars(signal) {
    if (signal == null || signal === '') return '';
    var n = Number(signal);
    if (isNaN(n)) return '';
    if (n >= -50) return '4'; // 4 barras
    if (n >= -60) return '3';
    if (n >= -70) return '2';
    if (n >= -80) return '1';
    return '0';
  }

  function fillNetworksList(networks) {
    const grid = document.getElementById('wizard-networks-grid');
    if (!grid) return;
    grid.innerHTML = '';
    if (!Array.isArray(networks) || networks.length === 0) {
      grid.innerHTML = '<p class="wizard-networks-empty text-muted">' + t('setup_wizard.select_network', d('Select a network', 'Selecciona una red')) + '</p>';
      return;
    }
    var escapeHtml = window.HostBerry && HostBerry.escapeHtml ? function(s) { return HostBerry.escapeHtml(s); } : function(s) { return s; };
    networks.forEach(function(net) {
      const ssid = net.ssid || net.SSID || '';
      if (!ssid) return;
      const isConnected = currentConnectedSSID && ssid === currentConnectedSSID;
      const signal = net.signal != null ? net.signal : net.signal_strength;
      const bars = signalBars(signal);
      const card = document.createElement('button');
      card.type = 'button';
      card.className = 'wizard-network-card' + (isConnected ? ' wizard-network-connected' : '');
      card.dataset.ssid = ssid;
      card.innerHTML =
        '<span class="wizard-network-icon"><i class="bi bi-wifi"></i><span class="wizard-signal-bars" data-bars="' + bars + '"></span></span>' +
        '<span class="d-flex align-items-center justify-content-center flex-wrap gap-1">' +
        '<span class="wizard-network-ssid">' + escapeHtml(ssid) + '</span>' +
        (isConnected ? '<span class="badge bg-success">' + t('setup_wizard.connected', d('Connected', 'Conectado')) + '</span>' : '') +
        '</span>' +
        (signal !== '' && signal != null ? '<span class="wizard-network-signal">' + signal + ' dBm</span>' : '');
      card.addEventListener('click', function() {
        selectedSSID = ssid;
        grid.querySelectorAll('.wizard-network-card').forEach(function(c) { c.classList.remove('selected'); });
        card.classList.add('selected');
        document.getElementById('wizard-wifi-password-box').classList.remove('d-none');
        document.getElementById('wizard-wifi-password').value = '';
        document.getElementById('wizard-wifi-password').focus();
      });
      grid.appendChild(card);
    });
  }

  function setupPasswordToggle(inputId, toggleBtnId) {
    var input = document.getElementById(inputId);
    var btn = document.getElementById(toggleBtnId);
    if (!input || !btn) return;
    var icon = btn.querySelector('i.bi');
    var showLabel = t('setup_wizard.show_password', d('Show password', 'Ver contraseña'));
    var hideLabel = t('setup_wizard.hide_password', d('Hide password', 'Ocultar contraseña'));
    btn.addEventListener('click', function() {
      if (input.type === 'password') {
        input.type = 'text';
        btn.title = hideLabel;
        btn.setAttribute('aria-label', hideLabel);
        if (icon) { icon.classList.remove('bi-eye'); icon.classList.add('bi-eye-slash'); }
      } else {
        input.type = 'password';
        btn.title = showLabel;
        btn.setAttribute('aria-label', showLabel);
        if (icon) { icon.classList.remove('bi-eye-slash'); icon.classList.add('bi-eye'); }
      }
    });
  }

  function showCurrentWifiBanner(ssid, connectionType) {
    var banner = document.getElementById('wizard-current-wifi-banner');
    var textEl = document.getElementById('wizard-current-wifi-text');
    var iconWrap = banner && banner.querySelector('.wizard-connection-icon');
    if (!banner || !textEl) return;
    currentConnectedSSID = ssid || null;
    var msg;
    if (connectionType === 'ethernet') {
      msg = t('setup_wizard.connected_via_cable', d('Connected via cable (Ethernet)', 'Conectado por cable (Ethernet)'));
      if (iconWrap) { iconWrap.innerHTML = '<i class="bi bi-ethernet me-2"></i>'; }
    } else {
      var wifiLabel = t('setup_wizard.connection_type_wifi', d('WiFi', 'WiFi'));
      var part = t('setup_wizard.connected_to_wifi', d('Connected to', 'Conectado a'));
      msg = (ssid ? part + ' ' + ssid + ' (' + wifiLabel + ')' : part + ' ' + wifiLabel).trim();
      if (iconWrap) { iconWrap.innerHTML = '<i class="bi bi-wifi me-2"></i>'; }
    }
    textEl.textContent = msg;
    banner.classList.remove('d-none');
  }

  function hideCurrentWifiBanner() {
    var banner = document.getElementById('wizard-current-wifi-banner');
    if (banner) banner.classList.add('d-none');
    currentConnectedSSID = null;
  }

  async function fetchWifiStatus() {
    try {
      var resp = await apiRequest('/api/v1/wifi/status', { method: 'GET' });
      if (!resp || !resp.ok) return;
      var data = await resp.json().catch(function() { return {}; });
      var ssid = data.ssid || data.current_connection || '';
      var connectionType = data.connection_type || '';
      if (connectionType === 'ethernet') {
        showCurrentWifiBanner(null, 'ethernet');
      } else if (ssid) {
        // Si tenemos SSID, mostramos la conexión por WiFi aunque el backend marque connected=false
        // (evita que no se muestre la red actual cuando el escaneo falla).
        showCurrentWifiBanner(ssid, 'wifi');
      } else if (data.connected) {
        // Conectado pero sin SSID detectado: mostramos WiFi genérico para que el usuario pueda continuar.
        showCurrentWifiBanner(null, 'wifi');
      }
    } catch (e) {}
  }

  async function scanNetworks() {
    const btn = document.getElementById('wizard-scan-btn');
    if (btn) {
      btn.disabled = true;
      btn.innerHTML = '<i class="bi bi-arrow-clockwise spinning me-2"></i>' + t('setup_wizard.scanning', d('Scanning...', 'Escaneando...'));
    }
    try {
      const resp = await apiRequest('/api/v1/wifi/scan', { method: 'POST' });
      const data = resp.ok ? await resp.json() : [];
      fillNetworksList(Array.isArray(data) ? data : (data.networks || []));
    } catch (e) {
      showAlert('danger', t('setup_wizard.error_scan', d('Error scanning networks', 'Error al buscar redes')));
      fillNetworksList([]);
    } finally {
      if (btn) {
        btn.disabled = false;
        btn.innerHTML = '<i class="bi bi-arrow-clockwise me-2"></i>' + t('setup_wizard.scan_networks', d('Scan networks', 'Buscar redes'));
      }
    }
  }

  async function connectWiFi() {
    if (!selectedSSID) return;
    const password = (document.getElementById('wizard-wifi-password') || {}).value || '';
    const connectBtn = document.getElementById('wizard-connect-btn');
    const btnText = connectBtn && connectBtn.querySelector('.btn-text');
    if (connectBtn) {
      connectBtn.disabled = true;
      if (btnText) btnText.textContent = t('setup_wizard.connecting', d('Connecting...', 'Conectando...'));
    }
    var timeoutId;
    var timeoutMs = 90000; // 90 s: el backend puede tardar hasta ~80 s (WPA + DHCP)
    var connectPromise = apiRequest('/api/v1/wifi/connect', {
      method: 'POST',
      body: { ssid: selectedSSID, password: password, country: 'ES' }
    });
    var timeoutPromise = new Promise(function(_, reject) {
      timeoutId = setTimeout(function() {
        var msg = t(
          'setup_wizard.error_connect_timeout',
          d(
            'Connection is taking too long. If the password is correct, wait 1-2 minutes and open the panel again (the device may have connected and have a new IP).',
            'La conexión tarda más de lo esperado. Si la contraseña es correcta, espera 1-2 minutos y abre de nuevo el panel (el dispositivo puede haberse conectado y tener otra IP).'
          )
        );
        reject(new Error(msg));
      }, timeoutMs);
    });
    try {
      const resp = await Promise.race([connectPromise, timeoutPromise]);
      clearTimeout(timeoutId);
      const data = await resp.json().catch(function() { return {}; });
      if (resp.ok && data.success !== false) {
        showAlert('success', t('setup_wizard.connected', d('Connected', 'Conectado')));
        setStep(2);
      } else {
        showAlert('danger', (data.error || t('setup_wizard.error_connect', d('Error connecting', 'Error al conectar'))));
      }
    } catch (e) {
      clearTimeout(timeoutId);
      var msg = e && e.message ? e.message : t('setup_wizard.error_connect', d('Error connecting', 'Error al conectar'));
      var isNetworkError = (e && (e.name === 'TypeError' || e.message === 'Failed to fetch' || (typeof e.message === 'string' && (e.message.indexOf('fetch') !== -1 || e.message.indexOf('NetworkError') !== -1))));
      if (isNetworkError) {
        if (selectedSSID) {
          msg = (t('setup_wizard.error_network_device_switching_ssid', d('The connection may have succeeded to «{ssid}». Wait about 30 seconds and open the panel again (the HostBerry may have a new IP on your WiFi).', 'La conexión puede haberse completado a «{ssid}». Espera unos 30 segundos y abre de nuevo el panel (el HostBerry puede tener una nueva IP en tu WiFi).')).replace(/\{ssid\}/g, selectedSSID));
        } else {
          msg = t('setup_wizard.error_network_device_switching', d('The connection may have succeeded, but the device is switching networks. Wait about 30 seconds and open the panel again (the HostBerry may have a new IP on your WiFi).', 'La conexión puede haberse completado, pero el dispositivo está cambiando de red. Espera unos 30 segundos y abre de nuevo el panel (el HostBerry puede tener una nueva IP en tu WiFi).'));
        }
        showAlert('warning', msg);

        // Verificación automática: aunque el fetch falle, la conexión puede haberse completado.
        // Intentamos consultar el estado durante un rato y avanzar si ya está conectado al SSID esperado.
        (async function verifyConnectionAfterSwitch() {
          var startTs = Date.now();
          var maxWaitMs = 120000; // 2 min
          var pollEveryMs = 3000;
          function normSSID(x) {
            try {
              return String(x || '')
                .replace(/^"+|"+$/g, '')
                .trim()
                .toLowerCase();
            } catch (_) {
              return '';
            }
          }

          while (Date.now() - startTs < maxWaitMs) {
            try {
              var r = await apiRequest('/api/v1/wifi/status', { method: 'GET' });
              if (r && r.ok) {
                var s = await r.json().catch(function() { return {}; });
                if (s && s.connection_type === 'ethernet') {
                  showAlert('success', t('setup_wizard.connected', d('Connected', 'Conectado')));
                  setStep(2);
                  return;
                }
                // WiFi: algunos firmwares/devices devuelven comillas o cambian el formato del SSID.
                // Validamos por SSID normalizado y confirmamos conexión real si existe.
                // Fallback: si ya estamos conectados por WiFi pero el backend no devuelve SSID,
                // igualmente avanzamos para que el wizard no se quede atascado.
                if (s && s.connection_type === 'wifi' && (s.connected === true || s.connected === undefined)) {
                  showAlert('success', t('setup_wizard.connected', d('Connected', 'Conectado')));
                  setStep(2);
                  return;
                }
                if (selectedSSID && s && normSSID(s.ssid) && normSSID(selectedSSID) === normSSID(s.ssid)) {
                  if (s.connected === true || s.connection_type === 'wifi') {
                    showAlert('success', t('setup_wizard.connected', d('Connected', 'Conectado')));
                    setStep(2);
                    return;
                  }
                }
              }
            } catch (_) {
              // Ignorar: en algunos momentos el hostberry ya cambió IP/red.
            }
            await new Promise(function(res) { setTimeout(res, pollEveryMs); });
          }

          // Si no se confirma conexión, dejamos el aviso original.
        })();
      } else {
        showAlert('danger', msg);
      }
    } finally {
      if (connectBtn) {
        connectBtn.disabled = false;
        if (btnText) btnText.textContent = t('setup_wizard.connect', d('Connect', 'Conectar'));
      }
    }
  }

  async function saveHostapd() {
    const ssid = (document.getElementById('wizard-ap-ssid') || {}).value || 'hostberry';
    const open = (document.getElementById('wizard-ap-open') || {}).checked;
    const password = (document.getElementById('wizard-ap-password') || {}).value.trim();
    const saveBtn = document.getElementById('wizard-next-2');
    if (!open && (password.length < 8 || password.length > 63)) {
      showAlert('warning', t('setup_wizard.ap_password_invalid', d('Password must be between 8 and 63 characters (WiFi WPA2/WPA3 standard).', 'La contraseña debe tener entre 8 y 63 caracteres (estándar WiFi WPA2/WPA3).')));
      return;
    }
    if (saveBtn) {
      saveBtn.disabled = true;
      saveBtn.querySelector('.btn-text').textContent = t('common.saving', d('Saving...', 'Guardando...'));
    }
    const payload = {
      interface: 'wlan0',
      ssid: ssid,
      channel: 6,
      security: open ? 'open' : 'wpa2',
      password: open ? '' : password,
      gateway: '192.168.4.1',
      dhcp_range_start: '192.168.4.2',
      dhcp_range_end: '192.168.4.254',
      lease_time: '12h',
      country: 'ES'
    };
    try {
      const resp = await apiRequest('/api/v1/hostapd/config', { method: 'POST', body: payload });
      const data = await resp.json().catch(function() { return {}; });
      if (resp.ok && !data.error) {
        showAlert('success', t('setup_wizard.success_ap', d('Access point configured successfully', 'Punto de acceso configurado correctamente')));
        setStep(3);
      } else {
        showAlert('danger', data.error || t('setup_wizard.error_save_ap', d('Error saving access point configuration', 'Error al guardar la configuración del punto de acceso')));
        if (saveBtn) { saveBtn.disabled = false; saveBtn.querySelector('.btn-text').textContent = t('setup_wizard.next', d('Next', 'Siguiente')); }
      }
    } catch (e) {
      showAlert('danger', t('setup_wizard.error_save_ap', d('Error saving access point configuration', 'Error al guardar la configuración del punto de acceso')));
      if (saveBtn) { saveBtn.disabled = false; saveBtn.querySelector('.btn-text').textContent = t('setup_wizard.next', d('Next', 'Siguiente')); }
    }
  }

  function init() {
    fetchWifiStatus();
    var scanBtn = document.getElementById('wizard-scan-btn');
    if (scanBtn) scanBtn.addEventListener('click', scanNetworks);
    var connectBtn = document.getElementById('wizard-connect-btn');
    if (connectBtn) connectBtn.addEventListener('click', connectWiFi);

    // Continuar (mantener conexión): avanza directo a paso 2
    var continueBtn = document.getElementById('wizard-continue-connected-btn');
    if (continueBtn) {
      continueBtn.addEventListener('click', function() {
        setStep(2);
      });
    }
    var back2 = document.getElementById('wizard-back-2');
    var next2 = document.getElementById('wizard-next-2');
    var back3 = document.getElementById('wizard-back-3');
    if (back2) back2.addEventListener('click', function() { setStep(1); });
    if (next2) next2.addEventListener('click', function() { saveHostapd(); });
    if (back3) back3.addEventListener('click', function() { setStep(2); });

    document.querySelectorAll('.wizard-security-option').forEach(function(card) {
      card.addEventListener('click', function() {
        selectedSecurityOption = card.getAttribute('data-option');
        document.querySelectorAll('.wizard-security-option').forEach(function(c) { c.classList.remove('border-primary', 'border-3'); });
        card.classList.add('border-primary', 'border-3');
        document.getElementById('wizard-go-config').disabled = false;
      });
    });
    var goConfig = document.getElementById('wizard-go-config');
    if (goConfig) goConfig.addEventListener('click', function() {
      if (selectedSecurityOption) window.location.href = '/setup-wizard/' + selectedSecurityOption;
    });

    setupPasswordToggle('wizard-wifi-password', 'wizard-wifi-toggle-pwd');
    setupPasswordToggle('wizard-ap-password', 'wizard-ap-toggle-pwd');

    var apOpen = document.getElementById('wizard-ap-open');
    if (apOpen) apOpen.addEventListener('change', function() {
      var box = document.getElementById('wizard-ap-password-box');
      if (box) box.classList.toggle('d-none', this.checked);
    });

    var params = new URLSearchParams(window.location.search || '');
    var stepParam = params.get('step');
    if (stepParam === '3') {
      setStep(3);
    } else {
      setStep(1);
    }
    scanNetworks();
  }

  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', init);
  } else {
    init();
  }
})();
