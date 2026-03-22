// JS para la página first-login con estética igual al login
(function(){
  // Cerrar dropdown de idioma al hacer clic fuera
  document.addEventListener('click', function(event) {
    const langDropdown = document.querySelector('.lang-dropdown');
    if (langDropdown && !langDropdown.contains(event.target)) {
      langDropdown.classList.remove('show');
    }
  });

  // Sistema de traducciones mejorado
  function t(key, defaultValue = '') {
    if (!key) return defaultValue || '';
    
    // 1. Primero intentar obtener del elemento i18n-data (atributos data-*)
    const i18nData = document.getElementById('i18n-data');
    if (i18nData) {
      const dataKey = key.replace(/\./g, '-');
      const value = i18nData.getAttribute(`data-${dataKey}`);
      if (value) {
        return value;
      }
    }
    
    // 2. Usar cache global (common.js) o JSON embebido si existe
    try {
      let translations = window._hbTranslations;
      const i18nJson = document.getElementById('i18n-json');
      if (i18nJson) {
        translations = JSON.parse(i18nJson.textContent || i18nJson.innerText || '{}');
      }
      if (translations && typeof translations === 'object') {
        const keys = String(key).split('.');
        let current = translations;
        for (const k of keys) {
          if (current && Object.prototype.hasOwnProperty.call(current, k)) {
            current = current[k];
          } else {
            break;
          }
        }
        if (typeof current === 'string') {
          return current;
        }
      }
    } catch (e) {
      // Ignorar errores de parsing
    }
    
    // 3. Fallback al sistema anterior (window.i18nData)
    const keys = key.split('.');
    let current = window.i18nData || {};
    
    for (const k of keys) {
      if (current && typeof current === 'object' && k in current) {
        current = current[k];
      } else {
        return defaultValue || key;
      }
    }
    
    return (typeof current === 'string' ? current : null) || defaultValue || key;
  }

  // Mismas notificaciones flotantes que el resto del panel: 8 s + botón cerrar (common.js)
  function showAlert(type, message) {
    if (window.HostBerry && typeof window.HostBerry.showAlert === 'function') {
      window.HostBerry.showAlert(type, message);
      return;
    }
    const ms = (window.HostBerry && window.HostBerry.NOTIFICATION_AUTO_DISMISS_MS) || 8000;
    const closeLabel = t('common.close', 'Close');
    const alertDiv = document.createElement('div');
    alertDiv.className = `alert alert-${type} alert-dismissible fade show position-fixed d-flex align-items-center`;
    alertDiv.style.cssText = 'top:20px;right:20px;z-index:9999;min-width:300px;max-width:400px;margin:0;';
    alertDiv.setAttribute('role', 'alert');
    const messageNode = document.createElement('span');
    messageNode.className = 'flex-grow-1 me-2';
    messageNode.textContent = String(message ?? '');
    const closeBtn = document.createElement('button');
    closeBtn.type = 'button';
    closeBtn.className = 'btn-close flex-shrink-0';
    closeBtn.setAttribute('aria-label', closeLabel);
    let tmr = null;
    const dismiss = () => {
      if (tmr) clearTimeout(tmr);
      alertDiv.remove();
    };
    closeBtn.addEventListener('click', (e) => { e.preventDefault(); dismiss(); });
    alertDiv.appendChild(messageNode);
    alertDiv.appendChild(closeBtn);
    document.body.appendChild(alertDiv);
    tmr = setTimeout(dismiss, ms);
  }

  // Función para mostrar notificación de éxito
  function showSuccess(message) {
    showAlert('success', message);
  }

  // Función para mostrar notificación de error
  function showError(message) {
    showAlert('danger', message);
  }

  // Función para mostrar notificación de información
  function showInfo(message) {
    showAlert('info', message);
  }
  
  // Función para mostrar notificación de advertencia
  function showWarning(message) {
    showAlert('warning', message);
  }
  
  // Función para mostrar notificaciones toast (compatibilidad)
  function showToast(title, message, type = 'info') {
    const alertType = type === 'success' ? 'success' : type === 'error' || type === 'danger' ? 'danger' : type === 'warning' ? 'warning' : 'info';
    showAlert(alertType, `${title}: ${message}`);
  }

  // Función para procesar errores de validación de Pydantic (con traducciones)
  function processValidationError(errorDetail) {
    if (Array.isArray(errorDetail)) {
      // Es un array de errores de validación de Pydantic
      const messages = errorDetail.map(error => {
        const field = error.loc && error.loc.length > 1 ? error.loc[1] : 'field';
        let message = error.msg || t('errors.validation_error', 'Error de validación');
        
        // Traducir nombres de campos
        const fieldNames = {
          'new_username': t('auth.username', 'Usuario'),
          'new_password': t('auth.password', 'Contraseña'),
          'confirm_password': t('auth.confirm_password', 'Confirmar contraseña')
        };
        
        // Traducir mensajes de error comunes de Pydantic
        const errorMessages = {
          'field required': t('errors.field_required', 'Este campo es requerido'),
          'string does not match expected pattern': t('errors.invalid_format', 'Formato inválido'),
          'string too short': t('errors.too_short', 'Demasiado corto'),
          'string too long': t('errors.too_long', 'Demasiado largo'),
          'value is not a valid string': t('errors.invalid_string', 'No es un texto válido'),
          'value is not a valid integer': t('errors.invalid_integer', 'No es un número válido'),
        };
        
        // Intentar traducir el mensaje de error
        const lowerMsg = message.toLowerCase();
        for (const [key, translation] of Object.entries(errorMessages)) {
          if (lowerMsg.includes(key)) {
            message = translation;
            break;
          }
        }
        
        // Si el mensaje contiene información sobre el campo, traducirlo
        const fieldName = fieldNames[field] || field;
        
        // Traducir mensajes específicos de validación
        if (message.includes('required')) {
          message = t('errors.field_required', 'Este campo es requerido');
        } else if (message.includes('too short') || message.includes('minimum')) {
          if (field === 'new_username') {
            message = t('errors.username_too_short', 'El nombre de usuario debe tener al menos 3 caracteres');
          } else if (field === 'new_password' || field === 'confirm_password') {
            message = t('errors.password_length', 'La contraseña debe tener al menos 8 caracteres');
          }
        } else if (message.includes('too long') || message.includes('maximum')) {
          if (field === 'new_username') {
            message = t('errors.username_too_long', 'El nombre de usuario no puede exceder 50 caracteres');
          }
        }
        
        return `${fieldName}: ${message}`;
      });
      
      return messages.join('\n');
    } else if (typeof errorDetail === 'string') {
      // Intentar traducir mensajes de error comunes
      const lowerMsg = errorDetail.toLowerCase();
      if (lowerMsg.includes('password') && lowerMsg.includes('match')) {
        return t('auth.passwords_dont_match', 'Las contraseñas no coinciden');
      }
      if (lowerMsg.includes('connection') || lowerMsg.includes('network')) {
        return t('errors.connection_error', 'Error de conexión');
      }
      if (lowerMsg.includes('validation')) {
        return t('errors.validation_error', 'Error de validación');
      }
      return errorDetail;
    } else if (typeof errorDetail === 'object') {
      if (errorDetail.message) {
        return processValidationError(errorDetail.message);
      }
      if (errorDetail.error) {
        return processValidationError(errorDetail.error);
      }
      return t('errors.validation_error', 'Error de validación');
    }
    
    return t('errors.validation_error', 'Error de validación');
  }

  function attachToggle(btnId, inputId, emojiId){
    const btn = document.getElementById(btnId);
    const input = document.getElementById(inputId);
    const emoji = document.getElementById(emojiId);
    if(!btn || !input || !emoji) return;
    
    btn.addEventListener('click', function(){
      const isPass = input.getAttribute('type') === 'password';
      input.setAttribute('type', isPass ? 'text' : 'password');
      emoji.textContent = isPass ? '🙈' : '👁️';
      const hideText = t('common.hide_password', 'Ocultar contraseña');
      const showText = t('common.show_password', 'Mostrar contraseña');
      emoji.setAttribute('title', isPass ? hideText : showText);
      btn.setAttribute('aria-label', isPass ? hideText : showText);
      btn.setAttribute('title', isPass ? hideText : showText);
    });
  }

  document.addEventListener('DOMContentLoaded', function(){
    // Configurar botones de mostrar/ocultar contraseña
    attachToggle('toggle-new-password', 'new_password', 'eye-emoji-new');
    attachToggle('toggle-confirm-password', 'confirm_password', 'eye-emoji-confirm');
    
    const form = document.getElementById('firstLoginForm');
    
    if(!form) return;
    
    form.addEventListener('submit', async function(e){
      e.preventDefault();
      const fd = new FormData(form);
      const payload = Object.fromEntries(fd.entries());
      
      if(payload.new_password !== payload.confirm_password){
        showError(t('auth.passwords_dont_match', 'Las contraseñas no coinciden'));
        return;
      }
      
      try{
        const currentLang = (document.documentElement && (document.documentElement.lang || document.documentElement.getAttribute('lang'))) || 'es';
        const headers = {
          'Content-Type': 'application/json',
          'Accept-Language': currentLang
        };

        const resp = await fetch('/api/v1/auth/first-login/change', {
          method: 'POST',
          headers: headers,
          credentials: 'include',
          body: JSON.stringify(payload)
        });
        
        let data = null;
        try {
          const text = await resp.text();
          if (text && text.trim()) {
            try {
              data = JSON.parse(text);
            } catch (_parseErr) {
              data = null;
            }
          }
        } catch (_jsonErr) {
          data = null;
        }

        if(resp && resp.ok){
          let successMessage = t('auth.credentials_updated_redirect', 'Credenciales actualizadas. Redirigiendo al dashboard.');
          if (data && typeof data === 'object' && data !== null && typeof data.message === 'string') {
            successMessage = data.message;
          } else if (typeof data === 'string') {
            successMessage = data;
          }
          if (typeof successMessage !== 'string') {
            successMessage = t('auth.credentials_updated_redirect', 'Credenciales actualizadas. Redirigiendo al dashboard.');
          }
          showSuccess(successMessage);
          setTimeout(function(){
            window.location.href = `/setup-wizard?lang=${encodeURIComponent(currentLang)}`;
          }, 1200);
          return;
        }

        const detail = data ? data.detail : null;
        if (detail) {
          showError(processValidationError(detail));
          return;
        }

        if (data && typeof data === 'object' && data !== null) {
          const hasTranslationKeys = data.adblock || data.auth || data.common || data.dashboard || data.errors;
          if (hasTranslationKeys) {
            data = null;
          }
        }

        if (resp && resp.status === 403 && data && typeof data === 'object' && data !== null && typeof data.error === 'string') {
          showError(data.error);
          setTimeout(function(){
            window.location.href = `/login?lang=${encodeURIComponent(currentLang)}`;
          }, 1200);
          return;
        }

        if (resp && resp.status === 401) {
          showError(t('auth.session_expired', 'Session expired'));
          setTimeout(function(){
            window.location.href = `/login?lang=${encodeURIComponent(currentLang)}`;
          }, 1200);
          return;
        }

        const status = resp && typeof resp.status === 'number' ? resp.status : 0;
        const baseMsg = t('errors.general_error_message', 'Ha ocurrido un error inesperado');
        showError(status ? `${baseMsg} (HTTP ${status})` : baseMsg);
      }catch(_e){
        const errorMsg = _e.message || t('errors.unknown_error', 'Error desconocido');
        showError(t('errors.connection_error', 'Error de conexión') + ': ' + errorMsg);
      }
	    });
	  });
	})();
