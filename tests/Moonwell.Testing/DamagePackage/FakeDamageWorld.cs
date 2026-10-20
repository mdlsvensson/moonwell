using System;
using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Testing
{
    /// <summary>The fake game the damage package's cases run against: it plays hits as the game does, events, armor, spell immunity and death included.</summary>
    public sealed class FakeDamageWorld : IDamageWorld
    {
        /// <summary>The damage types spell immunity stops, as the probe of 10-20 saw them.</summary>
        private static readonly int[] stopped = { 8, 9, 10, 13, 14, 15, 17, 18, 19, 20, 21, 24, 25 };

        /// <summary>The state of each unit the fake has been asked about.</summary>
        private readonly Dictionary<unit, FakeBody> bodies = new Dictionary<unit, FakeBody>();

        /// <summary>The events being played, innermost last.</summary>
        private readonly List<FakeEvent> events = new List<FakeEvent>();

        /// <summary>The package's damaging action, once it listens.</summary>
        private Action damaging = () => { };

        /// <summary>The package's damaged action, once it listens.</summary>
        private Action damaged = () => { };

        /// <summary>Whether the package asked to hear the game's damage events.</summary>
        public bool Listening;

        /// <summary>The action the pending 0 s timer will run; null when none is pending.</summary>
        public Action? Zero;

        /// <summary>When true, the next hit's after-armor event is withheld even without spell immunity, as for a case the package's table misses.</summary>
        public bool WithholdNextDamaged;

        /// <summary>The state of a unit, made with 100 life, no armor, normal defense, when first asked for.</summary>
        public FakeBody Body(unit target)
        {
            if (!bodies.TryGetValue(target, out var body))
            {
                body = new FakeBody();
                bodies[target] = body;
            }

            return body;
        }

        /// <summary>Plays a hit as the game does: damaging, the hits inside it, armor, damaged unless spell immunity stops it, then the life drops.</summary>
        public void Hit(unit? source, unit target, float amount, int attackType = 0, int damageType = 14, int weapon = 0, bool isAttack = false, Action? inside = null)
        {
            var current = new FakeEvent { Source = source, Target = target, Amount = amount, Attack = attackType, Damage = damageType, Weapon = weapon, IsAttack = isAttack };
            events.Add(current);
            if (Listening)
            {
                damaging();
            }

            if (inside != null)
            {
                inside();
            }

            var body = Body(target);
            var withheld = WithholdNextDamaged;
            WithholdNextDamaged = false;
            var stops = current.Attack == 4;
            for (var i = 0; i < stopped.Length; i++)
            {
                stops = stops || stopped[i] == current.Damage;
            }

            if (!(body.Immune && stops))
            {
                var armor = body.Armor + body.ArmorBonus;
                if (armor > 0f)
                {
                    current.Amount = current.Amount * (1f - 0.06f * armor / (1f + 0.06f * armor));
                }

                if (Listening && !withheld)
                {
                    damaged();
                }

                body.Life -= current.Amount;
                if (body.Life <= 0.405f)
                {
                    body.Life = 0f;
                    body.Dead = true;
                }
            }

            events.RemoveAt(events.Count - 1);
        }

        /// <summary>Runs the pending 0 s action; returns false when none was pending.</summary>
        public bool RunZero()
        {
            var action = Zero;
            Zero = null;
            if (action == null)
            {
                return false;
            }

            action();
            return true;
        }

        /// <summary>Remembers the package's actions.</summary>
        public void Listen(Action onDamaging, Action onDamaged)
        {
            Listening = true;
            damaging = onDamaging;
            damaged = onDamaged;
        }

        /// <summary>The current event's source.</summary>
        public unit? EventSource() => events[events.Count - 1].Source;

        /// <summary>The current event's target.</summary>
        public unit EventTarget() => events[events.Count - 1].Target;

        /// <summary>The current event's amount.</summary>
        public float EventAmount() => events[events.Count - 1].Amount;

        /// <summary>The current event's attack type.</summary>
        public int EventAttackType() => events[events.Count - 1].Attack;

        /// <summary>The current event's damage type.</summary>
        public int EventDamageType() => events[events.Count - 1].Damage;

        /// <summary>The current event's weapon type.</summary>
        public int EventWeaponType() => events[events.Count - 1].Weapon;

        /// <summary>Whether the current event is an attack.</summary>
        public bool EventIsAttack() => events[events.Count - 1].IsAttack;

        /// <summary>Sets the current event's amount.</summary>
        public void SetEventAmount(float amount) => events[events.Count - 1].Amount = amount;

        /// <summary>Sets the current event's attack type.</summary>
        public void SetEventAttackType(int type) => events[events.Count - 1].Attack = type;

        /// <summary>Sets the current event's damage type.</summary>
        public void SetEventDamageType(int type) => events[events.Count - 1].Damage = type;

        /// <summary>Sets the current event's weapon type.</summary>
        public void SetEventWeaponType(int type) => events[events.Count - 1].Weapon = type;

        /// <summary>A unit's life.</summary>
        public float Life(unit target) => Body(target).Life;

        /// <summary>Whether a unit is dead.</summary>
        public bool IsDead(unit target) => Body(target).Dead;

        /// <summary>Whether a unit is magic immune.</summary>
        public bool IsMagicImmune(unit target) => Body(target).Immune;

        /// <summary>Whether a unit counts as a melee attacker.</summary>
        public bool IsMelee(unit target) => Body(target).Melee;

        /// <summary>Whether a unit counts as a ranged attacker.</summary>
        public bool IsRanged(unit target) => Body(target).Ranged;

        /// <summary>A unit's armor with its bonus, as the game reads it.</summary>
        public float Armor(unit target) => Body(target).Armor + Body(target).ArmorBonus;

        /// <summary>Sets a unit's base armor, leaving its bonus, which is the harder of the two ways the game could read it.</summary>
        public void SetArmor(unit target, float armor) => Body(target).Armor = armor;

        /// <summary>A unit's defense type.</summary>
        public int DefenseType(unit target) => Body(target).Defense;

        /// <summary>Sets a unit's defense type.</summary>
        public void SetDefenseType(unit target, int type) => Body(target).Defense = type;

        /// <summary>Plays a hit from code; the game reads such a hit as no attack.</summary>
        public void Deal(unit source, unit target, float amount, bool attack, bool ranged, int attackType, int damageType, int weaponType)
        {
            Hit(source, target, amount, attackType, damageType, weaponType, false);
        }

        /// <summary>Keeps the action for RunZero.</summary>
        public void AfterZero(Action action)
        {
            Zero = action;
        }
    }

    /// <summary>A unit's state in the fake damage world.</summary>
    public sealed class FakeBody
    {
        /// <summary>The unit's life.</summary>
        public float Life = 100f;

        /// <summary>The unit's base armor.</summary>
        public float Armor;

        /// <summary>Armor from bonuses, which setting the armor leaves.</summary>
        public float ArmorBonus;

        /// <summary>The unit's defense type, as the game numbers it.</summary>
        public int Defense = 4;

        /// <summary>Whether the unit is magic immune.</summary>
        public bool Immune;

        /// <summary>Whether the unit counts as a melee attacker.</summary>
        public bool Melee;

        /// <summary>Whether the unit counts as a ranged attacker.</summary>
        public bool Ranged;

        /// <summary>Whether the unit is dead.</summary>
        public bool Dead;
    }

    /// <summary>One damage event the fake is playing.</summary>
    internal sealed class FakeEvent
    {
        /// <summary>The source.</summary>
        public unit? Source;

        /// <summary>The target.</summary>
        public unit Target = null!;

        /// <summary>The amount: before armor, then after.</summary>
        public float Amount;

        /// <summary>The attack type.</summary>
        public int Attack;

        /// <summary>The damage type.</summary>
        public int Damage;

        /// <summary>The weapon type.</summary>
        public int Weapon;

        /// <summary>Whether it is an attack.</summary>
        public bool IsAttack;
    }
}
