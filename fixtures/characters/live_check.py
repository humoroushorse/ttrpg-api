"""Builds each golden fixture as a real character through a running go_dnd, with the live catalog,
then checks the fixture's hand-checked values and diffs the main numbers against its golden sheet.
The characters are kept ("Fixture <name>"), so they can be opened in the UI; a rerun checks them again.

    TTRPG_USER=... TTRPG_PASSWORD=... python3 live_check.py [fixture ...]   (default: every fixture)
    DND_API=http://localhost:8004/api/v1 (the default)

Differences from a golden sheet are expected where a fixture's hand-written catalog parts are older
than the live data (no spellcasting, darkvision or species picks); the ETL-copied ones match.
"""

import glob
import http.cookiejar
import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request

BASE = os.environ.get('DND_API', 'http://localhost:8004/api/v1')
FIXTURES = os.path.dirname(os.path.abspath(__file__))
YEAR_2024 = {'dnd_version_year': {'filterType': 'date', 'type': 'equals', 'dateFrom': '2024-07-01'}}
KIND_ENTITY = {'species': 'species', 'background': 'backgrounds', 'subclass': 'subclasses', 'feat': 'feats', 'spell': 'spells'}

jar = http.cookiejar.CookieJar()
opener = urllib.request.build_opener(urllib.request.HTTPCookieProcessor(jar))


class ApiError(Exception):
    pass


def call(method, path, body=None, form=None):
    data, headers = None, {}
    if form is not None:
        data, headers = urllib.parse.urlencode(form).encode(), {'Content-Type': 'application/x-www-form-urlencoded'}
    elif body is not None:
        data, headers = json.dumps(body).encode(), {'Content-Type': 'application/json'}
    req = urllib.request.Request(f'{BASE}{path}', data=data, method=method, headers=headers)
    try:
        with opener.open(req) as r:
            text = r.read().decode()
            return json.loads(text) if text else None
    except urllib.error.HTTPError as e:
        detail = e.read().decode()[:300]
        raise ApiError(f'{method} {path} -> {e.code}: {detail}') from None


_ids = {}


def catalog_id(entity, name):
    key = (entity, name.lower())
    if key not in _ids:
        q = urllib.parse.urlencode({'name': name, 'limit': 50, 'fields': 'name', 'filter': json.dumps(YEAR_2024)})
        rows = [e for e in call('GET', f'/{entity}/query?{q}')['entities'] if e['name'].lower() == name.lower()]
        if len(rows) != 1:
            raise ApiError(f'{entity} "{name}": {len(rows)} 2024 matches')
        _ids[key] = rows[0]['id']
    return _ids[key]


def resolve(sheet, path):
    node = sheet
    for part in path.split('.'):
        if isinstance(node, list):
            node = node[int(part)]
        elif isinstance(node, dict):
            node = node.get(part)
        else:
            return None
    return node


def names_of(entries):
    out = set()
    for e in entries or []:
        if isinstance(e, dict):
            out.update(str(e[k]) for k in ('name', 'source', 'code', 'slot', 'spell') if e.get(k) is not None)
        else:
            out.add(str(e))
    return out


def check(sheet, exp):
    got = resolve(sheet, exp['path'])
    if 'equals' in exp:
        return got == exp['equals'], f"{exp['path']} = {json.dumps(got)}, want {json.dumps(exp['equals'])}"
    if 'has_name' in exp:
        return exp['has_name'] in names_of(got), f"{exp['path']} has {exp['has_name']!r}"
    if 'lacks_name' in exp:
        return exp['lacks_name'] not in names_of(got), f"{exp['path']} lacks {exp['lacks_name']!r}"
    return False, f'unknown expect {exp}'


def numbers(sheet):
    """The headline numbers, flat, for comparing two sheets"""
    out = {'pb': sheet['proficiency_bonus']['value'], 'ac': sheet['armor_class']['value'],
           'hp': sheet['hit_points']['max']['value'], 'init': sheet['initiative']['value']}
    for a, v in sheet['abilities'].items():
        out[f'ability.{a}'] = v['score']['value']
    for group in ('saving_throws', 'skills', 'speed', 'senses', 'passive'):
        for k, v in sheet[group].items():
            out[f'{group}.{k}'] = v['value']
    if sheet.get('spellcasting'):
        sc = sheet['spellcasting']
        out.update({'spell.dc': sc['save_dc']['value'], 'spell.attack': sc['attack']['value'], 'spell.slots': sc.get('slots')})
    out['features'] = len(sheet['features'])
    return out


def plan_levels(char_id, snap, planned):
    """Plans the snapshot's levels above `planned`, including those past its current level (planned ahead)"""
    class_id = catalog_id('classes', snap['class']['name'])
    top = max([snap['level']] + [l['level'] for l in snap['levels']])
    for lvl in range(planned + 1, top + 1):
        level = next((l for l in snap['levels'] if l['level'] == lvl), {})
        body = {'class_id': class_id, 'hp_method': level.get('hp_method', 'average')}
        if level.get('hp_value') is not None:
            body['hp_value'] = level['hp_value']
        call('PUT', f'/characters/{char_id}/levels/{lvl}', body)
    if top > planned and planned:
        print(f'   planned levels {planned + 1}-{top}')


def set_rules(char_id, snap):
    """The snapshot's optional rules on the character's own rules (the spellbook rule)"""
    if snap.get('spellbook_rule'):
        call('PATCH', f'/characters/{char_id}', {'rules': {'optional_rules': {'spellbook': True}}})


def sync(char_id, snap):
    """Brings an existing character in line: its rules, prepared spells (cantrips are picks now), the
    class's own spell picks, the starting equipment picks, and the snapshot's subclass"""
    set_rules(char_id, snap)
    detail = call('GET', f'/characters/{char_id}')
    plan_levels(char_id, snap, max([0] + [l['level'] for l in detail['levels']]))
    have = {p['spell_slug'] for p in detail['prepared_spells']}
    added = 0
    for spell in snap.get('prepared_spells') or []:
        spell_id = catalog_id('spells', spell['name'])
        if spell['name'].lower().replace(' ', '-') not in have:
            call('PUT', f'/characters/{char_id}/prepared-spells/{spell_id}')
            added += 1
    if added:
        print(f'   prepared {added} spells')
    for p in detail['prepared_spells']:
        if p['spell_level'] == 0:
            call('DELETE', f'/characters/{char_id}/prepared-spells/{p["spell_id"]}')
            print(f'   unprepared the cantrip {p["spell_name"]} (cantrips are picks now)')
    picked = {(c['slot'], (c.get('ref_name') or '').lower()) for c in detail['choices']}
    for c in snap['choices']:
        if c['kind'] == 'spell' and not c.get('granted_by') and (c['slot'], c['ref_name'].lower()) not in picked:
            body = {'level': c['level'], 'kind': 'spell', 'slot': c['slot'], 'spell_id': catalog_id('spells', c['ref_name'])}
            call('POST', f'/characters/{char_id}/choices', body)
            print(f'   picked {c["ref_name"]} ({c["slot"]})')
    # starting equipment (newer than the older fixture characters): the package, then its own picks
    real = {}  # fixture choice id -> real choice id
    by_slot = {c['slot']: c['id'] for c in detail['choices']}
    for c in snap['choices']:
        if c['kind'] != 'equipment_option':
            continue
        if c['slot'] in by_slot:
            real[c['id']] = by_slot[c['slot']]
            continue
        body = {'level': c['level'], 'kind': c['kind'], 'slot': c['slot'], 'value': c['value']}
        if c.get('granted_by'):
            body['granted_by_id'] = real[c['granted_by']]
        after = call('POST', f'/characters/{char_id}/choices', body)
        real[c['id']] = [x for x in after['choices'] if x['slot'] == c['slot']][-1]['id']
        print(f'   equipment {c["slot"]}: {c["value"]}')
    if snap.get('subclass'):
        want = catalog_id('subclasses', snap['subclass']['name'])
        for c in detail['choices']:
            if c['kind'] == 'subclass' and c['subclass_id'] != want:
                body = {k: c[k] for k in ('level', 'kind', 'slot') if c.get(k) is not None}
                body['subclass_id'] = want
                call('PUT', f'/characters/{char_id}/choices/{c["id"]}', body)
                print(f'   subclass {c.get("ref_name")} -> {snap["subclass"]["name"]}')


def build(stem):
    fixture = json.load(open(f'{FIXTURES}/{stem}.json'))
    snap = fixture['snapshot']
    name = f'Fixture {stem}'
    existing = {c['name']: c['id'] for c in call('GET', '/characters/query?limit=200')['entities']}
    if name in existing:
        print(f'{name}: exists ({existing[name]}), checking only')
        char_id = existing[name]
    else:
        char_id = call('POST', '/characters', {'name': name, 'current_level': snap['level']})['id']
        set_rules(char_id, snap)
        call('PUT', f'/characters/{char_id}/ability-scores', {'method': snap['ability_method'], 'scores': snap['base_scores']})
        plan_levels(char_id, snap, 0)
        made = {}  # fixture choice id -> real choice id
        for c in snap['choices']:
            body = {'level': c['level'], 'kind': c['kind'], 'slot': c['slot']}
            if c.get('value') is not None:
                body['value'] = c['value']
            if c.get('granted_by'):
                body['granted_by_id'] = made[c['granted_by']]
            if c['kind'] in KIND_ENTITY:
                body[f"{c['kind']}_id"] = catalog_id(KIND_ENTITY[c['kind']], c['ref_name'])
            detail = call('POST', f'/characters/{char_id}/choices', body)
            new = [x for x in detail['choices'] if x['id'] not in made.values()]
            made[c['id']] = new[-1]['id']
        for item in snap['items']:
            body = {'item_id': catalog_id('items', item['name']), 'equipped': item.get('equipped', False),
                    'attuned': item.get('attuned', False), 'quantity': item.get('quantity', 1)}
            if item.get('level_received') is not None:
                body['level_received'] = item['level_received']
            call('POST', f'/characters/{char_id}/items', body)
        for m in snap['modifiers']:
            body = {k: m[k] for k in ('label', 'target', 'op', 'value', 'from_level', 'active', 'added_as') if k in m}
            call('POST', f'/characters/{char_id}/modifiers', body)
        print(f'{name}: built ({char_id}): {len(snap["choices"])} choices, {len(snap["items"])} items, {len(snap["modifiers"])} modifiers')
    sync(char_id, snap)

    golden = json.load(open(f'{FIXTURES}/{stem}.sheet.json'))
    failed = 0
    for lvl in fixture['levels']:
        sheet = call('GET', f'/characters/{char_id}/sheet?level={lvl}')
        for exp in [e for e in fixture['expect'] if e.get('level', lvl) == lvl]:
            ok, msg = check(sheet, exp)
            if not ok:
                failed += 1
                print(f'   L{lvl} EXPECT FAILED {msg}')
        want, got = numbers(golden[str(lvl)]), numbers(sheet)
        diffs = [f'{k} {want.get(k)}->{got.get(k)}' for k in sorted(set(want) | set(got)) if want.get(k) != got.get(k)]
        open_points = [f"{p['slot']} {p['chosen']}/{p['count']}" for p in sheet['choice_points'] if p['chosen'] < p['count']]
        issues = [f"{i['severity']}:{i['code']}" for i in sheet['issues']]
        print(f'   L{lvl}: vs golden {"same" if not diffs else "; ".join(diffs)}')
        if open_points:
            print(f'        open choice points: {", ".join(open_points)}')
        if issues:
            print(f'        issues: {", ".join(issues)}')
    passed = len(fixture['expect']) - failed
    print(f'   expects: {passed}/{len(fixture["expect"])} pass')
    return failed


def main():
    call('POST', '/auth/session/token', form={'username': os.environ['TTRPG_USER'], 'password': os.environ['TTRPG_PASSWORD']})
    stems = sys.argv[1:] or sorted(os.path.basename(f)[:-5] for f in glob.glob(f'{FIXTURES}/*.json') if not f.endswith('.sheet.json'))
    failed = 0
    for stem in stems:
        try:
            failed += build(stem)
        except ApiError as e:
            failed += 1
            print(f'Fixture {stem}: STOPPED {e}')
    print('ALL EXPECTS PASS' if not failed else f'{failed} failures')


main()
