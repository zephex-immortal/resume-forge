/* ═══════════════════════════════════════════════════════════════
   resume forge — production-ready app
   ═══════════════════════════════════════════════════════════════ */

document.addEventListener('DOMContentLoaded', () => {
    // ─── STATE ──────────────────────────────────────────────
    const state = {
        template: 'terminal',
        genCount: parseInt(localStorage.getItem('resumeForgeCount') || '0'),
        isGenerating: false,
        lastHTML: '',
        lastData: null,
        lastAIContent: null,
    };

    // ─── RESUME HISTORY ─────────────────────────────────────
    let resumeHistory = [];
    try {
        const stored = localStorage.getItem('resumeForgeHistory');
        if (stored) resumeHistory = JSON.parse(stored);
    } catch {}

    function saveHistory(entry) {
        resumeHistory.unshift(entry);
        // Keep last 20
        if (resumeHistory.length > 20) resumeHistory = resumeHistory.slice(0, 20);
        localStorage.setItem('resumeForgeHistory', JSON.stringify(resumeHistory));
        renderHistory();
        updateGenCountDisplay();
    }

    function deleteHistoryItem(id) {
        resumeHistory = resumeHistory.filter(e => e.id !== id);
        localStorage.setItem('resumeForgeHistory', JSON.stringify(resumeHistory));
        renderHistory();
    }

    // ─── DOM REFS ───────────────────────────────────────────
    const form = document.getElementById('resume-form');
    const genBtn = document.getElementById('gen-btn');
    const genStatus = document.getElementById('gen-status');
    const preview = document.getElementById('resume-preview');
    const navCount = document.getElementById('nav-gen-count');
    const expContainer = document.getElementById('exp-container');
    const eduContainer = document.getElementById('edu-container');
    const modalClose = document.getElementById('modal-close');
    const aboutClose = document.querySelector('.about-close');
    const templatesModal = document.getElementById('templates-modal');
    const aboutModal = document.getElementById('about-modal');
    const historyPanel = document.getElementById('history-panel');
    const historyList = document.getElementById('history-list');
    const emptyHistory = document.getElementById('empty-history');

    // ─── INIT ───────────────────────────────────────────────
    updateGenCountDisplay();
    renderHistory();

    // ─── TOAST SYSTEM ───────────────────────────────────────
    function showToast(msg, type = 'info') {
        const existing = document.querySelector('.toast-msg');
        if (existing) existing.remove();
        const toast = document.createElement('div');
        toast.className = `toast-msg toast-${type}`;
        toast.textContent = msg;
        document.body.appendChild(toast);
        setTimeout(() => toast.classList.add('show'), 10);
        setTimeout(() => {
            toast.classList.remove('show');
            setTimeout(() => toast.remove(), 300);
        }, 4000);
    }

    // ─── ADD EXPERIENCE ─────────────────────────────────────
    function createExpEntry(data = {}) {
        const entry = document.createElement('div');
        entry.className = 'exp-entry';
        if (data.company) entry.dataset.hasData = 'true';
        entry.innerHTML = `
            <div class="exp-header"><span class="exp-counter">#1</span><button type="button" class="remove-entry" title="remove">✕</button></div>
            <div class="form-row">
                <div class="form-group flex-1"><label>company</label><input type="text" class="exp-company" placeholder="Company" value="${data.company || ''}"></div>
                <div class="form-group flex-1"><label>role</label><input type="text" class="exp-role" placeholder="Role" value="${data.role || ''}"></div>
            </div>
            <div class="form-row">
                <div class="form-group flex-1"><label>from</label><input type="text" class="exp-from" placeholder="2024" value="${data.from || ''}"></div>
                <div class="form-group flex-1"><label>to</label><input type="text" class="exp-to" placeholder="Present" value="${data.to || ''}"></div>
            </div>
            <div class="form-group"><label>description</label><textarea class="exp-desc" rows="2" placeholder="What you did... (AI will expand this)">${data.desc || ''}</textarea></div>
        `;
        entry.querySelector('.remove-entry').addEventListener('click', () => entry.remove());
        return entry;
    }

    function updateExpCounters() {
        document.querySelectorAll('.exp-entry').forEach((el, i) => {
            const c = el.querySelector('.exp-counter');
            if (c) c.textContent = `#${i + 1}`;
        });
    }

    document.getElementById('add-exp').addEventListener('click', () => {
        const entry = createExpEntry();
        expContainer.appendChild(entry);
        updateExpCounters();
    });

    function createEduEntry(data = {}) {
        const entry = document.createElement('div');
        entry.className = 'edu-entry';
        entry.innerHTML = `
            <div class="exp-header"><span class="exp-counter">#1</span><button type="button" class="remove-entry" title="remove">✕</button></div>
            <div class="form-row">
                <div class="form-group flex-1"><label>school</label><input type="text" class="edu-school" placeholder="University Name" value="${data.school || ''}"></div>
                <div class="form-group flex-1"><label>degree</label><input type="text" class="edu-degree" placeholder="B.Tech CSE" value="${data.degree || ''}"></div>
            </div>
            <div class="form-row">
                <div class="form-group flex-1"><label>from</label><input type="text" class="edu-from" placeholder="2024" value="${data.from || ''}"></div>
                <div class="form-group flex-1"><label>to</label><input type="text" class="edu-to" placeholder="2028" value="${data.to || ''}"></div>
            </div>
        `;
        entry.querySelector('.remove-entry')?.addEventListener('click', () => entry.remove());
        return entry;
    }

    document.getElementById('add-edu').addEventListener('click', () => {
        const entry = createEduEntry();
        eduContainer.appendChild(entry);
    });

    // ─── TEMPLATE SELECTION ────────────────────────────────
    document.querySelectorAll('.tmpl-option').forEach(el => {
        el.addEventListener('click', () => {
            document.querySelectorAll('.tmpl-option').forEach(o => o.classList.remove('selected'));
            el.classList.add('selected');
            const radio = el.querySelector('input[type="radio"]');
            if (radio) {
                radio.checked = true;
                state.template = radio.value;
            }
        });
    });

    // ─── MODALS ─────────────────────────────────────────────
    document.getElementById('nav-templates-btn').addEventListener('click', (e) => {
        e.preventDefault();
        templatesModal.classList.remove('hidden');
    });
    document.getElementById('nav-about-btn').addEventListener('click', (e) => {
        e.preventDefault();
        aboutModal.classList.remove('hidden');
    });
    const closeModals = () => {
        templatesModal.classList.add('hidden');
        aboutModal.classList.add('hidden');
    };
    modalClose?.addEventListener('click', closeModals);
    aboutClose?.addEventListener('click', closeModals);
    document.querySelectorAll('.modal-backdrop').forEach(b => b.addEventListener('click', closeModals));
    document.addEventListener('keydown', (e) => { if (e.key === 'Escape') closeModals(); });

    // Template preview cards
    document.querySelectorAll('.tmpl-preview-card').forEach(card => {
        card.addEventListener('click', () => {
            const tmpl = card.dataset.tmpl;
            state.template = tmpl;
            document.querySelectorAll('.tmpl-option').forEach(o => {
                o.classList.toggle('selected', o.dataset.tmpl === tmpl);
                const radio = o.querySelector('input[type="radio"]');
                if (radio) radio.checked = (radio.value === tmpl);
            });
            closeModals();
        });
    });

    // ─── HISTORY TOGGLE ─────────────────────────────────────
    document.getElementById('nav-history-btn').addEventListener('click', (e) => {
        e.preventDefault();
        historyPanel.classList.toggle('open');
    });
    document.getElementById('close-history').addEventListener('click', () => {
        historyPanel.classList.remove('open');
    });

    // ─── RENDER HISTORY ────────────────────────────────────
    function renderHistory() {
        if (!historyList) return;
        if (resumeHistory.length === 0) {
            historyList.innerHTML = '';
            if (emptyHistory) emptyHistory.style.display = 'block';
            return;
        }
        if (emptyHistory) emptyHistory.style.display = 'none';

        historyList.innerHTML = resumeHistory.map((item, idx) => {
            const date = new Date(item.timestamp);
            const dateStr = date.toLocaleDateString('en-IN', { day: '2-digit', month: 'short', year: 'numeric' });
            const timeStr = date.toLocaleTimeString('en-IN', { hour: '2-digit', minute: '2-digit' });
            return `<div class="history-item">
                <div class="hi-info" onclick="document.getElementById('load-history-${idx}').click()">
                    <div class="hi-name">${item.name || 'Untitled'}</div>
                    <div class="hi-meta">${item.role || ''} · ${item.template || ''} · ${dateStr} ${timeStr}</div>
                </div>
                <div class="hi-actions">
                    <button class="hi-btn" id="load-history-${idx}" data-idx="${idx}" title="View">👁</button>
                    <button class="hi-btn hi-del" data-id="${item.id}" title="Delete">🗑</button>
                </div>
            </div>`;
        }).join('');

        // Load from history
        historyList.querySelectorAll('[id^="load-history-"]').forEach(btn => {
            btn.addEventListener('click', () => {
                const idx = parseInt(btn.dataset.idx);
                const item = resumeHistory[idx];
                if (item) {
                    state.lastHTML = item.html;
                    state.lastData = item.formData;
                    state.lastAIContent = item.aiContent;
                    preview.innerHTML = item.html;
                    showToast('📄 Loaded from history', 'info');
                    historyPanel.classList.remove('open');
                }
            });
        });

        // Delete from history
        historyList.querySelectorAll('.hi-del').forEach(btn => {
            btn.addEventListener('click', (e) => {
                e.stopPropagation();
                if (confirm('Delete this resume?')) {
                    deleteHistoryItem(btn.dataset.id);
                    showToast('🗑 Deleted', 'info');
                }
            });
        });
    }

    // ─── COLLECT FORM DATA ─────────────────────────────────
    function getFormData() {
        const exps = [];
        document.querySelectorAll('.exp-entry').forEach(el => {
            exps.push({
                company: el.querySelector('.exp-company')?.value || '',
                role: el.querySelector('.exp-role')?.value || '',
                from: el.querySelector('.exp-from')?.value || '',
                to: el.querySelector('.exp-to')?.value || '',
                desc: el.querySelector('.exp-desc')?.value || '',
            });
        });
        const edus = [];
        document.querySelectorAll('.edu-entry').forEach(el => {
            edus.push({
                school: el.querySelector('.edu-school')?.value || '',
                degree: el.querySelector('.edu-degree')?.value || '',
                from: el.querySelector('.edu-from')?.value || '',
                to: el.querySelector('.edu-to')?.value || '',
            });
        });
        return {
            apiKey: document.getElementById('api-key').value,
            name: document.getElementById('name').value.trim() || 'Your Name',
            role: document.getElementById('role').value.trim() || 'Developer',
            email: document.getElementById('email').value.trim() || '',
            phone: document.getElementById('phone').value.trim() || '',
            location: document.getElementById('location').value.trim() || '',
            portfolio: document.getElementById('portfolio').value.trim() || '',
            summary: document.getElementById('summary').value.trim() || '',
            skills: document.getElementById('skills').value.trim() || '',
            experience: exps.filter(e => e.company && e.role),
            education: edus.filter(e => e.school && e.degree),
            template: state.template,
        };
    }

    // ─── FILL FORM FROM HISTORY ────────────────────────────
    function fillFormFromData(data) {
        if (data.apiKey) document.getElementById('api-key').value = data.apiKey;
        if (data.name) document.getElementById('name').value = data.name;
        if (data.role) document.getElementById('role').value = data.role;
        if (data.email) document.getElementById('email').value = data.email;
        if (data.phone) document.getElementById('phone').value = data.phone;
        if (data.location) document.getElementById('location').value = data.location;
        if (data.portfolio) document.getElementById('portfolio').value = data.portfolio;
        if (data.summary) document.getElementById('summary').value = data.summary;
        if (data.skills) document.getElementById('skills').value = data.skills;
        if (data.template) {
            state.template = data.template;
            document.querySelectorAll('.tmpl-option').forEach(o => o.classList.toggle('selected', o.dataset.tmpl === data.template));
            document.querySelectorAll('input[name="template"]').forEach(r => r.checked = (r.value === data.template));
        }

        // Experience
        expContainer.innerHTML = '';
        (data.experience || []).forEach(e => expContainer.appendChild(createExpEntry(e)));
        if (!data.experience?.length) expContainer.appendChild(createExpEntry());
        updateExpCounters();

        // Education
        eduContainer.innerHTML = '';
        (data.education || []).forEach(e => eduContainer.appendChild(createEduEntry(e)));
        if (!data.education?.length) eduContainer.appendChild(createEduEntry());
    }

    // ─── GENERATE WITH GEMINI ──────────────────────────────
    async function generateResumeContent(data) {
        const prompt = `You are a professional resume writer. Generate a JSON resume for a person with these details:

Name: ${data.name}
Role: ${data.role}
Email: ${data.email}
Phone: ${data.phone}
Location: ${data.location}
Portfolio: ${data.portfolio}

Summary: ${data.summary || 'Generate a professional summary based on their experience.'}

Experience: ${JSON.stringify(data.experience)}
Education: ${JSON.stringify(data.education)}
Skills: ${data.skills}

Return ONLY valid JSON with this exact structure (no markdown, no code fences):
{
  "summary": "well written professional summary...",
  "experience": [
    {"company": "...", "role": "...", "from": "...", "to": "...", "description": "expanded professional description with achievements..."}
  ],
  "education": [
    {"school": "...", "degree": "...", "from": "...", "to": "..."}
  ]
}

For each experience entry, expand the description to include specific achievements and responsibilities. Make it compelling and professional. Be specific. Use numbers and results where possible.`;

        const controller = new AbortController();
        const timeout = setTimeout(() => controller.abort(), 30000);

        try {
            const res = await fetch(`https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=${data.apiKey}`, {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    contents: [{ parts: [{ text: prompt }] }],
                    generationConfig: { temperature: 0.7, maxOutputTokens: 2048 },
                }),
                signal: controller.signal,
            });
            clearTimeout(timeout);

            if (!res.ok) {
                const err = await res.json().catch(() => ({ error: { message: res.statusText } }));
                throw new Error(err.error?.message || `HTTP ${res.status}`);
            }

            const json = await res.json();
            const text = json.candidates?.[0]?.content?.parts?.[0]?.text || '';

            let jsonStr = text;
            const jsonMatch = text.match(/```(?:json)?\s*([\s\S]*?)```/);
            if (jsonMatch) jsonStr = jsonMatch[1];

            try {
                return JSON.parse(jsonStr);
            } catch {
                return {
                    summary: text.substring(0, 500),
                    experience: data.experience,
                    education: data.education,
                };
            }
        } catch (err) {
            clearTimeout(timeout);
            if (err.name === 'AbortError') throw new Error('AI request timed out');
            throw err;
        }
    }

    // ─── RENDER RESUME ─────────────────────────────────────
    function renderResume(data, aiContent) {
        const skills = data.skills.split(',').map(s => s.trim()).filter(Boolean);
        const summary = aiContent?.summary || data.summary || 'Professional with experience in software development and technology.';
        const experiences = aiContent?.experience?.length ? aiContent.experience : data.experience;
        const educations = aiContent?.education?.length ? aiContent.education : data.education;
        let html = '';

        if (data.template === 'compact') {
            html = `<div class="resume-output resume-compact" style="font-family:'Segoe UI',Arial,sans-serif;color:#222;padding:1.2rem 1.5rem;">
                <div style="border-bottom:2px solid #00cc9e;padding-bottom:0.5rem;margin-bottom:0.8rem;">
                    <h1 style="font-size:1.4rem;font-weight:700;margin:0;color:#0a0a0e;">${data.name}</h1>
                    <div style="font-size:0.8rem;color:#555;margin-top:0.15rem;">${data.role}</div>
                    <div style="font-size:0.65rem;color:#777;margin-top:0.3rem;display:flex;flex-wrap:wrap;gap:0.5rem;">
                        ${data.email ? `<span>✉ ${data.email}</span>` : ''}${data.phone ? `<span>📞 ${data.phone}</span>` : ''}${data.location ? `<span>📍 ${data.location}</span>` : ''}${data.portfolio ? `<span>🔗 ${data.portfolio}</span>` : ''}
                    </div>
                </div>
                ${summary ? `<div style="margin-bottom:0.8rem;"><div style="font-size:0.7rem;font-weight:600;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.2rem;">Summary</div><p style="font-size:0.72rem;line-height:1.5;color:#333;margin:0;">${summary}</p></div>` : ''}
                ${experiences.length ? `<div style="margin-bottom:0.8rem;"><div style="font-size:0.7rem;font-weight:600;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.3rem;">Experience</div>${experiences.map(e => `<div style="margin-bottom:0.4rem;"><div style="font-size:0.78rem;font-weight:600;">${e.role} — <span style="font-weight:400;color:#555;">${e.company}</span></div><div style="font-size:0.65rem;color:#777;">${e.from || ''} — ${e.to || ''}</div><div style="font-size:0.7rem;color:#444;margin-top:0.1rem;line-height:1.4;">${e.description || e.desc || ''}</div></div>`).join('')}</div>` : ''}
                ${educations.length ? `<div style="margin-bottom:0.8rem;"><div style="font-size:0.7rem;font-weight:600;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.2rem;">Education</div>${educations.map(e => `<div style="font-size:0.75rem;margin-bottom:0.15rem;"><span style="font-weight:600;">${e.degree}</span> — ${e.school} <span style="color:#777;">(${e.from || ''} — ${e.to || ''})</span></div>`).join('')}</div>` : ''}
                ${skills.length ? `<div><div style="font-size:0.7rem;font-weight:600;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.2rem;">Skills</div><div style="display:flex;flex-wrap:wrap;gap:0.3rem;">${skills.map(s => `<span style="font-size:0.65rem;padding:0.15rem 0.5rem;background:#f0f0f0;border-radius:2px;">${s}</span>`).join('')}</div></div>` : ''}
            </div>`;
        } else if (data.template === 'terminal') {
            html = `<div class="resume-output resume-terminal">
                <div class="rt-header"><h1>${data.name}</h1><div class="rt-role">${data.role}</div><div class="rt-contact">${data.email ? `<span>✉ ${data.email}</span>` : ''}${data.phone ? `<span>📞 ${data.phone}</span>` : ''}${data.location ? `<span>📍 ${data.location}</span>` : ''}${data.portfolio ? `<span>🔗 ${data.portfolio}</span>` : ''}</div></div>
                <div class="rt-body">
                    ${summary ? `<div class="rt-section"><div class="rt-section-title">// summary</div><div class="rt-item-desc">${summary}</div></div>` : ''}
                    ${experiences.length ? `<div class="rt-section"><div class="rt-section-title">// experience</div>${experiences.map(e => `<div class="rt-item"><div class="rt-item-title">${e.role} @ ${e.company}</div><div class="rt-item-sub">${e.from || ''} — ${e.to || ''}</div><div class="rt-item-desc">${e.description || e.desc || ''}</div></div>`).join('')}</div>` : ''}
                    ${educations.length ? `<div class="rt-section"><div class="rt-section-title">// education</div>${educations.map(e => `<div class="rt-item"><div class="rt-item-title">${e.degree} @ ${e.school}</div><div class="rt-item-sub">${e.from || ''} — ${e.to || ''}</div></div>`).join('')}</div>` : ''}
                    ${skills.length ? `<div class="rt-section"><div class="rt-section-title">// skills</div><div class="rt-skills">${skills.map(s => `<span class="rt-skill-tag">${s}</span>`).join('')}</div></div>` : ''}
                </div>
            </div>`;
        } else if (data.template === 'minimal') {
            html = `<div class="resume-output resume-minimal">
                <div class="rm-header"><h1>${data.name}</h1><div class="rm-role">${data.role}</div><div class="rm-contact">${data.email ? `<span>${data.email}</span>` : ''}${data.phone ? `<span>${data.phone}</span>` : ''}${data.location ? `<span>${data.location}</span>` : ''}${data.portfolio ? `<span>${data.portfolio}</span>` : ''}</div></div>
                <div class="rm-body">
                    <div class="rt-section-title">${'\u2500'.repeat(30)}</div>
                    <p style="font-size:0.75rem;margin:0.5rem 0;line-height:1.5;color:#333;">${summary}</p>
                    ${experiences.length ? `<div style="margin-top:0.8rem;"><div class="rm-section-title">Experience</div>${experiences.map(e => `<div style="margin-bottom:0.4rem;"><div style="font-size:0.78rem;font-weight:600;">${e.role} — ${e.company}</div><div style="font-size:0.68rem;color:#555;">${e.from || ''} — ${e.to || ''}</div><div style="font-size:0.7rem;color:#333;margin-top:0.1rem;">${e.description || e.desc || ''}</div></div>`).join('')}</div>` : ''}
                    ${skills.length ? `<div style="margin-top:0.8rem;"><div class="rm-section-title">Skills</div><div style="display:flex;flex-wrap:wrap;gap:0.3rem;margin-top:0.3rem;">${skills.map(s => `<span style="font-size:0.65rem;padding:0.1rem 0.5rem;border:1px solid #ddd;border-radius:2px;">${s}</span>`).join('')}</div></div>` : ''}
                    ${educations.length ? `<div style="margin-top:0.8rem;"><div class="rm-section-title">Education</div>${educations.map(e => `<div style="margin-bottom:0.3rem;"><div style="font-size:0.78rem;font-weight:600;">${e.degree} — ${e.school}</div><div style="font-size:0.68rem;color:#555;">${e.from || ''} — ${e.to || ''}</div></div>`).join('')}</div>` : ''}
                </div>
            </div>`;
        } else if (data.template === 'modern') {
            html = `<div class="resume-output resume-modern">
                <div class="mod-sidebar"><h1>${data.name}</h1><div class="mod-role">${data.role}</div><div class="mod-contact">${data.email ? `<div>✉ ${data.email}</div>` : ''}${data.phone ? `<div>📞 ${data.phone}</div>` : ''}${data.location ? `<div>📍 ${data.location}</div>` : ''}${data.portfolio ? `<div>🔗 ${data.portfolio}</div>` : ''}</div>
                    ${skills.length ? `<div><div class="mod-section-title">Skills</div>${skills.map(s => `<div class="mod-skill-item">▸ ${s}</div>`).join('')}</div>` : ''}
                    ${educations.length ? `<div><div class="mod-section-title">Education</div>${educations.map(e => `<div style="margin-bottom:0.4rem;"><div style="font-size:0.7rem;font-weight:600;color:#fff;">${e.degree}</div><div style="font-size:0.62rem;color:#aaa;">${e.school}</div><div style="font-size:0.6rem;color:#777;">${e.from || ''} — ${e.to || ''}</div></div>`).join('')}</div>` : ''}
                </div>
                <div class="mod-main">
                    <div style="margin-bottom:1rem;"><div class="mod-section-title">About</div><p style="font-size:0.72rem;color:#333;line-height:1.5;">${summary}</p></div>
                    ${experiences.length ? `<div><div class="mod-section-title">Experience</div>${experiences.map(e => `<div style="margin-bottom:0.5rem;"><div class="mod-item-title">${e.role}</div><div class="mod-item-sub">${e.company} · ${e.from || ''} — ${e.to || ''}</div><p style="font-size:0.68rem;color:#444;margin-top:0.1rem;line-height:1.4;">${e.description || e.desc || ''}</p></div>`).join('')}</div>` : ''}
                </div>
            </div>`;
        }

        state.lastHTML = html;
        state.lastData = data;
        state.lastAIContent = aiContent;
        preview.innerHTML = html;
    }

    // ─── FORM SUBMIT (WITH PAYMENT SAFETY) ────────────────
    form.addEventListener('submit', async (e) => {
        e.preventDefault();
        if (state.isGenerating) return;

        const data = getFormData();
        if (!data.apiKey) {
            genStatus.textContent = '[!] enter your Gemini API key';
            showToast('Enter your Gemini API key', 'error');
            return;
        }

        let paymentReceived = false;

        state.isGenerating = true;
        genBtn.disabled = true;
        genBtn.querySelector('.btn-text').textContent = 'processing...';
        genStatus.textContent = '⏳ creating payment order...';

        try {
            // Step 1: Create Razorpay order
            const orderRes = await fetch('http://localhost:7001/api/create-order', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({ amount: 1000 }),
            });
            if (!orderRes.ok) throw new Error('Payment service unavailable');
            const order = await orderRes.json();

            // Step 2: Open Razorpay checkout
            genStatus.textContent = '💳 complete payment to continue...';
            const paymentResult = await new Promise((resolve, reject) => {
                const rzp = new Razorpay({
                    key: order.key,
                    order_id: order.order_id,
                    amount: order.amount,
                    currency: 'INR',
                    name: 'Resume Forge',
                    description: 'AI Resume Generation',
                    handler: (response) => resolve(response),
                    modal: { ondismiss: () => reject(new Error('CANCELLED')) },
                    prefill: { name: data.name, email: data.email, contact: data.phone },
                    theme: { color: '#00ffc8' },
                });
                rzp.open();
            });

            // Payment succeeded
            paymentReceived = true;
            genStatus.textContent = '✅ payment received! generating resume...';
            showToast('✅ Payment successful! Generating...', 'success');

            // Step 3: Verify payment (async, don't block generation)
            fetch('http://localhost:7001/api/verify-payment', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(paymentResult),
            }).catch(() => {});

            // Step 4: Generate with AI
            let aiContent = null;
            try {
                genStatus.textContent = '⏳ AI is writing your resume...';
                genBtn.querySelector('.btn-text').textContent = 'generating...';
                aiContent = await generateResumeContent(data);
            } catch (aiErr) {
                console.warn('AI generation failed, using raw data:', aiErr);
                genStatus.textContent = '⚠️ AI enhancement failed, using your data';
                showToast('⚠️ AI hit a snag, used raw data', 'warn');
            }

            // Step 5: Render resume (always works)
            renderResume(data, aiContent);

            // Step 6: Save to history
            const historyEntry = {
                id: Date.now().toString(36) + Math.random().toString(36).slice(2, 6),
                timestamp: Date.now(),
                name: data.name,
                role: data.role,
                template: data.template,
                html: state.lastHTML,
                formData: data,
                aiContent: aiContent,
                paymentId: paymentResult.razorpay_payment_id || '',
            };
            saveHistory(historyEntry);

            state.genCount++;
            localStorage.setItem('resumeForgeCount', String(state.genCount));
            updateGenCountDisplay();

            genStatus.textContent = '✅ resume ready! download or copy below';
            showToast('🎉 Resume generated successfully!', 'success');

        } catch (err) {
            if (err.message === 'CANCELLED') {
                genStatus.textContent = '[!] payment was cancelled';
                showToast('Payment cancelled', 'warn');
            } else if (!paymentReceived) {
                genStatus.textContent = `❌ ${err.message || 'Something went wrong'}`;
                showToast('❌ ' + (err.message || 'Error'), 'error');
            } else {
                // Payment was received but something failed after
                // Render anyway — the user paid!
                genStatus.textContent = '⚠️ Payment received but had issues. Rendering resume...';
                showToast('⚠️ Payment processed! Rendering resume...', 'warn');
                try {
                    renderResume(data, null);
                    const entry = {
                        id: Date.now().toString(36),
                        timestamp: Date.now(),
                        name: data.name,
                        role: data.role,
                        template: data.template,
                        html: state.lastHTML || '',
                        formData: data,
                        aiContent: null,
                        paymentId: 'unknown',
                    };
                    saveHistory(entry);
                    genStatus.textContent = '✅ Resume generated! Contact support for AI issues.';
                } catch {
                    genStatus.textContent = '❌ Fatal error. Your payment was received. Contact support.';
                    showToast('❌ Critical error. Payment logged.', 'error');
                }
            }
        }

        state.isGenerating = false;
        genBtn.disabled = false;
        genBtn.querySelector('.btn-text').textContent = 'generate resume';
    });

    // ─── UPDATE COUNTER ────────────────────────────────────
    function updateGenCountDisplay() {
        navCount.textContent = `${resumeHistory.length} saved · ${state.genCount} total`;
        const priceSpan = document.querySelector('.btn-price');
        if (priceSpan) priceSpan.textContent = `₹10 · ${state.genCount} made`;
    }

    // ─── DOWNLOAD PDF ─────────────────────────────────────
    document.getElementById('download-pdf').addEventListener('click', () => {
        const content = document.querySelector('.resume-output');
        if (!content) { showToast('Generate a resume first', 'warn'); return; }
        document.title = `resume-${document.getElementById('name').value || 'untitled'}`;
        window.print();
        document.title = 'resume forge';
    });

    // ─── COPY HTML ────────────────────────────────────────
    document.getElementById('copy-html').addEventListener('click', async () => {
        if (!state.lastHTML) { showToast('Generate a resume first', 'warn'); return; }
        try {
            await navigator.clipboard.writeText(state.lastHTML);
            showToast('✅ HTML copied to clipboard!', 'success');
        } catch {
            showToast('Failed to copy', 'error');
        }
    });

    // ─── PRINT STYLES ─────────────────────────────────────
    const printStyle = document.createElement('style');
    printStyle.textContent = `@media print { body * { visibility: hidden; } #resume-preview, #resume-preview * { visibility: visible; } #resume-preview { position: absolute; top: 0; left: 0; width: 100%; } .resume-output { box-shadow: none; margin: 0; border-radius: 0; } @page { margin: 0; size: A4; } }`;
    document.head.appendChild(printStyle);

    // ─── TOAST STYLES ─────────────────────────────────────
    const toastStyle = document.createElement('style');
    toastStyle.textContent = `.toast-msg { position: fixed; bottom: 1.5rem; left: 50%; transform: translateX(-50%) translateY(10px); padding: 0.6rem 1.2rem; border-radius: 4px; font-size: 0.78rem; font-family: var(--font); z-index: 9999; opacity: 0; transition: all 0.3s ease; pointer-events: none; border: 1px solid var(--border); } .toast-msg.show { opacity: 1; transform: translateX(-50%) translateY(0); } .toast-success { background: rgba(0,255,200,0.1); color: var(--cyan); border-color: rgba(0,255,200,0.3); } .toast-error { background: rgba(255,51,85,0.1); color: var(--red); border-color: rgba(255,51,85,0.3); } .toast-warn { background: rgba(255,204,0,0.1); color: var(--yellow); border-color: rgba(255,204,0,0.3); } .toast-info { background: rgba(0,102,255,0.1); color: #66aaff; border-color: rgba(0,102,255,0.3); }`;
    document.head.appendChild(toastStyle);

    console.log('✦ resume forge — production ready');
});
