# Mobile Accounting Audit Tracker

## Maqsad

Mobile app va `mobile_server` ichida count, qty, status va wording joylari
bir-biriga to'liq mos bo'lsin.

Yakunda:
- home summary
- status breakdown
- status detail
- recent
- notifications

hammasi bir xil truth source bilan ishlasin.

## Hozir tugagan ishlar

### 1. Werka create flow

- katta item bazada customer assignment lookup fix qilindi
- Werka customer issue create ishlaydigan qilindi
- birinchi create sekinligi qisqartirildi

Commitlar:
- `533de39` `Fix werka customer issue assignment lookup for large item sets`
- `323ff7c` `Cache delivery note state field checks`
- `abd97ec` `Warm werka delivery note path on startup`

### 2. Werka status math

- confirmed breakdown `0 Kg` bo'lib chiqishi tuzatildi
- delivery note accepted qty mapping to'g'rilandi
- breakdown/detail canonical mappingga qaytarildi

Commitlar:
- `e32d313` `Fix werka breakdown quantities for delivery notes`
- `d4e25fa` `Fix werka status breakdown math and fallback`
- `7d3f3fc` `Route werka status views through canonical mappings`

### 3. Werka wording

- breakdown card ichidagi `receipt` wording generic qilindi

Commit:
- `3e3db76` `Use generic record wording in werka breakdown`

### 4. Server stability

- `core` va `tunnel` self-heal yo'liga tushirildi
- watchdog timer qo'shildi
- global SSH uchun Tailscale yo'li tayyorlandi

Commitlar:
- `2d21147` `Harden mobile server systemd recovery`
- `166780c` `Add mobile server watchdog timer`
- `686deb1` `Fix mobile server systemd pre-start cleanup`

## Hali qolgan audit

### Werka

- [ ] `home summary` sonlari `breakdown/detail` bilan to'liq bir xilmi
- [ ] `recent` ichidagi status va qty lar `detail` bilan mosmi
- [ ] `notifications` ichidagi matnlar real qty bilan mosmi
- [ ] `pending/confirmed/returned` countlari app runtime store tufayli drift bermayaptimi

### Supplier

- [ ] supplier home summary audit
- [ ] supplier status breakdown/detail audit
- [ ] supplier recent/notifications audit

### Customer

- [ ] customer home summary audit
- [ ] customer status detail va response natijalari audit
- [ ] customer notification metrics audit

### Admin

- [ ] admin supplier summary audit
- [ ] admin activity count/qty wording audit

## Tekshiruv qoidasi

Har bir audit shu tartibda yuradi:

1. app ekranidagi ko'rinish
2. mobile API response
3. server mapping
4. ERP truth source

Shu to'rtalasi mos kelmasa, fix qilinadi.

## Truth source qoidasi

- Werka customer delivery flow uchun truth source: `Delivery Note`
- Werka supplier intake flow uchun truth source: `Purchase Receipt`
- app ichidagi runtime store faqat UI smoothing uchun
- final hisob-kitob server truth source'dan kelishi kerak

## Keyingi birinchi ish

Avval Werka oqimini to'liq yopamiz:

- [ ] Werka `home summary` vs `breakdown/detail`
- [ ] Werka `recent`
- [ ] Werka `notifications`

Shundan keyin Supplier oqimiga o'tamiz.

## Eslatma

Yangi workaround qo'shilmaydi.

Eski buglarni yopish usuli:
- bitta source of truth
- bitta mapping
- bitta wording

## Status

Hozirgi umumiy holat:
- Werka eng og'riqli buglari: katta qismi yopilgan
- butun app bo'ylab behato hisob audit: hali tugamagan
