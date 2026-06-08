/* ═══════════════════════════════════════════════════════════════
   resume forge — app
   ═══════════════════════════════════════════════════════════════ */

document.addEventListener('DOMContentLoaded', () => {
    // ─── STATE ──────────────────────────────────────────────
    const state = {
        template: 'terminal',
        genCount: parseInt(localStorage.getItem('resumeForgeCount') || '0'),
        isGenerating: false,
        lastHTML: '',
    };

    // ─── DOM REFS ───────────────────────────────────────────
    const form = document.getElementById('resume-form');
    const genBtn = document.getElementById('gen-btn');
    const genStatus = document.getElementById('gen-status');
    const preview = document.getElementById('resume-preview');
    const navCount = document.getElementById('nav-gen-count');
    const expContainer = document.getElementById('exp-container');
    const eduContainer = document.getElementById('edu-container');
    const templateRadios = document.querySelectorAll('input[name="template"]');
    const modalClose = document.getElementById('modal-close');
    const aboutClose = document.querySelector('.about-close');
    const templatesModal = document.getElementById('templates-modal');
    const aboutModal = document.getElementById('about-modal');

    // ─── INIT ───────────────────────────────────────────────
    navCount.textContent = `${state.genCount} generated`;
    updateGenCountDisplay();

    // ─── ADD EXPERIENCE ─────────────────────────────────────
    document.getElementById('add-exp').addEventListener('click', () => {
        const entry = document.createElement('div');
        entry.className = 'exp-entry';
        entry.style.marginTop = '0.6rem';
        entry.style.paddingTop = '0.6rem';
        entry.style.borderTop = '1px solid rgba(30,30,50,0.2)';
        entry.innerHTML = `
            <div class="form-row">
                <div class="form-group flex-1"><label>company</label><input type="text" class="exp-company" placeholder="Company"></div>
                <div class="form-group flex-1"><label>role</label><input type="text" class="exp-role" placeholder="Role"></div>
            </div>
            <div class="form-row">
                <div class="form-group flex-1"><label>from</label><input type="text" class="exp-from" placeholder="2024"></div>
                <div class="form-group flex-1"><label>to</label><input type="text" class="exp-to" placeholder="Present"></div>
            </div>
            <div class="form-group"><label>description</label><textarea class="exp-desc" rows="2" placeholder="What you did..."></textarea></div>
            <button type="button" class="btn-sm remove-entry" style="color:var(--red);border-color:rgba(255,51,85,0.3);margin-top:0.2rem;">✕ remove</button>
        `;
        entry.querySelector('.remove-entry').addEventListener('click', () => entry.remove());
        expContainer.appendChild(entry);
    });

    document.getElementById('add-edu').addEventListener('click', () => {
        const entry = document.createElement('div');
        entry.className = 'edu-entry';
        entry.style.marginTop = '0.6rem';
        entry.style.paddingTop = '0.6rem';
        entry.style.borderTop = '1px solid rgba(30,30,50,0.2)';
        entry.innerHTML = `
            <div class="form-row">
                <div class="form-group flex-1"><label>school</label><input type="text" class="edu-school" placeholder="School"></div>
                <div class="form-group flex-1"><label>degree</label><input type="text" class="edu-degree" placeholder="Degree"></div>
            </div>
            <div class="form-row">
                <div class="form-group flex-1"><label>from</label><input type="text" class="edu-from" placeholder="2024"></div>
                <div class="form-group flex-1"><label>to</label><input type="text" class="edu-to" placeholder="2028"></div>
            </div>
            <button type="button" class="btn-sm remove-entry" style="color:var(--red);border-color:rgba(255,51,85,0.3);margin-top:0.2rem;">✕ remove</button>
        `;
        entry.querySelector('.remove-entry').addEventListener('click', () => entry.remove());
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
            name: document.getElementById('name').value || 'Your Name',
            role: document.getElementById('role').value || 'Developer',
            email: document.getElementById('email').value || '',
            phone: document.getElementById('phone').value || '',
            location: document.getElementById('location').value || '',
            portfolio: document.getElementById('portfolio').value || '',
            summary: document.getElementById('summary').value || '',
            skills: document.getElementById('skills').value || '',
            experience: exps.filter(e => e.company && e.role),
            education: edus.filter(e => e.school && e.degree),
            template: state.template,
        };
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

For each experience entry, expand the description to include specific achievements and responsibilities if none provided. Make it compelling and professional.`;

        const res = await fetch(`https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=${data.apiKey}`, {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({
                contents: [{ parts: [{ text: prompt }] }],
                generationConfig: { temperature: 0.7, maxOutputTokens: 2048 },
            }),
        });

        if (!res.ok) {
            const err = await res.json().catch(() => ({ error: { message: res.statusText } }));
            throw new Error(err.error?.message || `HTTP ${res.status}`);
        }

        const json = await res.json();
        const text = json.candidates?.[0]?.content?.parts?.[0]?.text || '';

        // Extract JSON from response (handle markdown fences)
        let jsonStr = text;
        const jsonMatch = text.match(/```(?:json)?\s*([\s\S]*?)```/);
        if (jsonMatch) jsonStr = jsonMatch[1];

        try {
            return JSON.parse(jsonStr);
        } catch {
            // Fallback: return structured content from the text itself
            return {
                summary: text.substring(0, 500),
                experience: data.experience,
                education: data.education,
            };
        }
    }

    // ─── RENDER RESUME ─────────────────────────────────────
    function renderResume(data, aiContent) {
        const skills = data.skills.split(',').map(s => s.trim()).filter(Boolean);
        const summary = aiContent?.summary || data.summary || 'Professional with experience in the field.';
        const experiences = aiContent?.experience?.length ? aiContent.experience : data.experience;
        const educations = aiContent?.education?.length ? aiContent.education : data.education;

        let html = '';

        if (data.template === 'compact') {
            html = `<div class="resume-output resume-compact" style="font-family:'Segoe UI',Arial,sans-serif;color:#222;padding:1.2rem 1.5rem;">
                <div style="border-bottom:2px solid #00cc9e;padding-bottom:0.5rem;margin-bottom:0.8rem;">
                    <h1 style="font-size:1.4rem;font-weight:700;margin:0;color:#0a0a0e;">${data.name}</h1>
                    <div style="font-size:0.8rem;color:#555;margin-top:0.15rem;">${data.role}</div>
                    <div style="font-size:0.65rem;color:#777;margin-top:0.3rem;display:flex;flex-wrap:wrap;gap:0.5rem;">
                        ${data.email ? `<span>${data.email}</span>` : ''}
                        ${data.phone ? `<span>${data.phone}</span>` : ''}
                        ${data.location ? `<span>${data.location}</span>` : ''}
                        ${data.portfolio ? `<span>${data.portfolio}</span>` : ''}
                    </div>
                </div>
                ${summary ? `<div style="margin-bottom:0.8rem;">
                    <div style="font-size:0.7rem;font-weight:600;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.2rem;">Summary</div>
                    <p style="font-size:0.72rem;line-height:1.5;color:#333;margin:0;">${summary}</p>
                </div>` : ''}
                ${experiences.length ? `<div style="margin-bottom:0.8rem;">
                    <div style="font-size:0.7rem;font-weight:600;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.3rem;">Experience</div>
                    ${experiences.map(e => `<div style="margin-bottom:0.4rem;">
                        <div style="font-size:0.78rem;font-weight:600;">${e.role} — <span style="font-weight:400;color:#555;">${e.company}</span></div>
                        <div style="font-size:0.65rem;color:#777;">${e.from || ''} — ${e.to || ''}</div>
                        <div style="font-size:0.7rem;color:#444;margin-top:0.1rem;line-height:1.4;">${e.description || e.desc || ''}</div>
                    </div>`).join('')}
                </div>` : ''}
                ${educations.length ? `<div style="margin-bottom:0.8rem;">
                    <div style="font-size:0.7rem;font-weight:600;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.2rem;">Education</div>
                    ${educations.map(e => `<div style="font-size:0.75rem;margin-bottom:0.15rem;"><span style="font-weight:600;">${e.degree}</span> — ${e.school} <span style="color:#777;">(${e.from || ''} — ${e.to || ''})</span></div>`).join('')}
                </div>` : ''}
                ${skills.length ? `<div>
                    <div style="font-size:0.7rem;font-weight:600;color:#00cc9e;text-transform:uppercase;letter-spacing:1px;margin-bottom:0.2rem;">Skills</div>
                    <div style="display:flex;flex-wrap:wrap;gap:0.3rem;">${skills.map(s => `<span style="font-size:0.65rem;padding:0.15rem 0.5rem;background:#f0f0f0;border-radius:2px;">${s}</span>`).join('')}</div>
                </div>` : ''}
            </div>`;
        } else if (data.template === 'terminal') {
            html = `<div class="resume-output resume-terminal">
                <div class="rt-header">
                    <h1>${data.name}</h1>
                    <div class="rt-role">${data.role}</div>
                    <div class="rt-contact">
                        ${data.email ? `<span>✉ ${data.email}</span>` : ''}
                        ${data.phone ? `<span>📞 ${data.phone}</span>` : ''}
                        ${data.location ? `<span>📍 ${data.location}</span>` : ''}
                        ${data.portfolio ? `<span>🔗 ${data.portfolio}</span>` : ''}
                    </div>
                </div>
                <div class="rt-body">
                    <div class="rt-section">
                        <div class="rt-section-title">// summary</div>
                        <div class="rt-item-desc">${summary}</div>
                    </div>
                    ${experiences.length ? `<div class="rt-section">
                        <div class="rt-section-title">// experience</div>
                        ${experiences.map(e => `<div class="rt-item">
                            <div class="rt-item-title">${e.role} @ ${e.company}</div>
                            <div class="rt-item-sub">${e.from || ''} — ${e.to || ''}</div>
                            <div class="rt-item-desc">${e.description || e.desc || ''}</div>
                        </div>`).join('')}
                    </div>` : ''}
                    ${educations.length ? `<div class="rt-section">
                        <div class="rt-section-title">// education</div>
                        ${educations.map(e => `<div class="rt-item">
                            <div class="rt-item-title">${e.degree} @ ${e.school}</div>
                            <div class="rt-item-sub">${e.from || ''} — ${e.to || ''}</div>
                        </div>`).join('')}
                    </div>` : ''}
                    ${skills.length ? `<div class="rt-section">
                        <div class="rt-section-title">// skills</div>
                        <div class="rt-skills">${skills.map(s => `<span class="rt-skill-tag">${s}</span>`).join('')}</div>
                    </div>` : ''}
                </div>
            </div>`;
        } else if (data.template === 'minimal') {
            html = `<div class="resume-output resume-minimal">
                <div class="rm-header">
                    <h1>${data.name}</h1>
                    <div class="rm-role">${data.role}</div>
                    <div class="rm-contact">
                        ${data.email ? `<span>${data.email}</span>` : ''}
                        ${data.phone ? `<span>${data.phone}</span>` : ''}
                        ${data.location ? `<span>${data.location}</span>` : ''}
                        ${data.portfolio ? `<span>${data.portfolio}</span>` : ''}
                    </div>
                </div>
                <div class="rm-body">
                    <div class="rt-section-title">${'\u2500'.repeat(30)}</div>
                    <p style="font-size:0.75rem;margin:0.5rem 0;line-height:1.5;color:#333;">${summary}</p>
                    ${experiences.length ? `<div style="margin-top:0.8rem;"><div class="rm-section-title">Experience</div>
                        ${experiences.map(e => `<div style="margin-bottom:0.4rem;">
                            <div style="font-size:0.78rem;font-weight:600;">${e.role} — ${e.company}</div>
                            <div style="font-size:0.68rem;color:#555;">${e.from || ''} — ${e.to || ''}</div>
                            <div style="font-size:0.7rem;color:#333;margin-top:0.1rem;">${e.description || e.desc || ''}</div>
                        </div>`).join('')}</div>` : ''}
                    ${skills.length ? `<div style="margin-top:0.8rem;"><div class="rm-section-title">Skills</div>
                        <div style="display:flex;flex-wrap:wrap;gap:0.3rem;margin-top:0.3rem;">
                            ${skills.map(s => `<span style="font-size:0.65rem;padding:0.1rem 0.5rem;border:1px solid #ddd;border-radius:2px;">${s}</span>`).join('')}
                        </div>
                    </div>` : ''}
                    ${educations.length ? `<div style="margin-top:0.8rem;"><div class="rm-section-title">Education</div>
                        ${educations.map(e => `<div style="margin-bottom:0.3rem;">
                            <div style="font-size:0.78rem;font-weight:600;">${e.degree} — ${e.school}</div>
                            <div style="font-size:0.68rem;color:#555;">${e.from || ''} — ${e.to || ''}</div>
                        </div>`).join('')}</div>` : ''}
                </div>
            </div>`;
        } else if (data.template === 'modern') {
            html = `<div class="resume-output resume-modern">
                <div class="mod-sidebar">
                    <h1>${data.name}</h1>
                    <div class="mod-role">${data.role}</div>
                    <div class="mod-contact">
                        ${data.email ? `<div>✉ ${data.email}</div>` : ''}
                        ${data.phone ? `<div>📞 ${data.phone}</div>` : ''}
                        ${data.location ? `<div>📍 ${data.location}</div>` : ''}
                        ${data.portfolio ? `<div>🔗 ${data.portfolio}</div>` : ''}
                    </div>
                    ${skills.length ? `<div>
                        <div class="mod-section-title">Skills</div>
                        ${skills.map(s => `<div class="mod-skill-item">▸ ${s}</div>`).join('')}
                    </div>` : ''}
                    ${educations.length ? `<div>
                        <div class="mod-section-title">Education</div>
                        ${educations.map(e => `<div style="margin-bottom:0.4rem;">
                            <div style="font-size:0.7rem;font-weight:600;color:#fff;">${e.degree}</div>
                            <div style="font-size:0.62rem;color:#aaa;">${e.school}</div>
                            <div style="font-size:0.6rem;color:#777;">${e.from || ''} — ${e.to || ''}</div>
                        </div>`).join('')}
                    </div>` : ''}
                </div>
                <div class="mod-main">
                    <div style="margin-bottom:1rem;">
                        <div class="mod-section-title">About</div>
                        <p style="font-size:0.72rem;color:#333;line-height:1.5;">${summary}</p>
                    </div>
                    ${experiences.length ? `<div>
                        <div class="mod-section-title">Experience</div>
                        ${experiences.map(e => `<div style="margin-bottom:0.5rem;">
                            <div class="mod-item-title">${e.role}</div>
                            <div class="mod-item-sub">${e.company} · ${e.from || ''} — ${e.to || ''}</div>
                            <p style="font-size:0.68rem;color:#444;margin-top:0.1rem;line-height:1.4;">${e.description || e.desc || ''}</p>
                        </div>`).join('')}
                    </div>` : ''}
                </div>
            </div>`;
        }

        state.lastHTML = html;
        preview.innerHTML = html;
    }

    // ─── FORM SUBMIT ──────────────────────────────────────
    form.addEventListener('submit', async (e) => {
        e.preventDefault();
        if (state.isGenerating) return;

        const data = getFormData();
        if (!data.apiKey) {
            genStatus.textContent = '[!] enter your Gemini API key';
            return;
        }

        state.isGenerating = true;
        genBtn.disabled = true;
        genStatus.textContent = '⏳ generating...';
        genBtn.querySelector('.btn-text').textContent = 'generating...';

        try {
            const aiContent = await generateResumeContent(data);
            renderResume(data, aiContent);

            state.genCount++;
            localStorage.setItem('resumeForgeCount', String(state.genCount));
            updateGenCountDisplay();

            genStatus.textContent = '✅ resume generated!';
        } catch (err) {
            genStatus.textContent = `❌ ${err.message || 'generation failed'}`;
            // Still render with raw data even if AI fails
            renderResume(data, null);
        }

        state.isGenerating = false;
        genBtn.disabled = false;
        genBtn.querySelector('.btn-text').textContent = 'generate resume';
    });

    function updateGenCountDisplay() {
        // Also update the generate button to show counter
        const priceSpan = document.querySelector('.btn-price');
        if (priceSpan) {
            priceSpan.textContent = `₹10 · ${state.genCount} made`;
        }
    }

    // ─── DOWNLOAD PDF ─────────────────────────────────────
    document.getElementById('download-pdf').addEventListener('click', () => {
        const content = document.querySelector('.resume-output');
        if (!content) {
            genStatus.textContent = '[!] generate a resume first';
            return;
        }
        // Use print to PDF
        const originalTitle = document.title;
        document.title = `resume-${document.getElementById('name').value || 'untitled'}`;
        window.print();
        document.title = originalTitle;
    });

    // ─── COPY HTML ────────────────────────────────────────
    document.getElementById('copy-html').addEventListener('click', async () => {
        if (!state.lastHTML) {
            genStatus.textContent = '[!] generate a resume first';
            return;
        }
        try {
            await navigator.clipboard.writeText(state.lastHTML);
            genStatus.textContent = '✅ HTML copied to clipboard!';
        } catch {
            genStatus.textContent = '[!] failed to copy';
        }
    });

    // ─── PRINT STYLES ─────────────────────────────────────
    const printStyle = document.createElement('style');
    printStyle.textContent = `
        @media print {
            body * { visibility: hidden; }
            #resume-preview, #resume-preview * { visibility: visible; }
            #resume-preview { position: absolute; top: 0; left: 0; width: 100%; }
            .resume-output { box-shadow: none; margin: 0; }
            @page { margin: 0; size: A4; }
        }
    `;
    document.head.appendChild(printStyle);

    console.log('✦ resume forge ready');
});
