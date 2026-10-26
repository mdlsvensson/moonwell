using System;
using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell.Engine
{
    /// <summary>The game as the damage package sees it, through the game's natives; the only class of the package that calls them.</summary>
    internal sealed class NativeDamageWorld : IDamageWorld
    {
        /// <summary>The timer of AfterZero, made on first use.</summary>
        private timer? zero;

        /// <summary>The action the running 0 s timer will run; null when it is not running.</summary>
        private Action? zeroAction;

        /// <summary>The game's attack types by number, 0 to 6, made once in Listen: a hit's type is looked up here, with no native call.</summary>
        private readonly List<attacktype> attackTypes = new List<attacktype>();

        /// <summary>The game's damage types by number, 0 to 26, made once in Listen.</summary>
        private readonly List<damagetype> damageTypes = new List<damagetype>();

        /// <summary>The game's weapon types by number, 0 to 23, made once in Listen.</summary>
        private readonly List<weapontype> weaponTypes = new List<weapontype>();

        /// <summary>Makes the type lists, then registers both damage events for every player slot.</summary>
        public void Listen(Action damaging, Action damaged)
        {
            for (var i = 0; i <= 6; i++)
            {
                attackTypes.Add(Natives.ConvertAttackType(i));
            }

            for (var i = 0; i <= 26; i++)
            {
                damageTypes.Add(Natives.ConvertDamageType(i));
            }

            for (var i = 0; i <= 23; i++)
            {
                weaponTypes.Add(Natives.ConvertWeaponType(i));
            }

            Register(Natives.EVENT_PLAYER_UNIT_DAMAGING, damaging);
            Register(Natives.EVENT_PLAYER_UNIT_DAMAGED, damaged);
        }

        /// <summary>The current damage event's source.</summary>
        public unit? EventSource() => Natives.GetEventDamageSource();

        /// <summary>The current damage event's target.</summary>
        public unit EventTarget() => Natives.BlzGetEventDamageTarget();

        /// <summary>The current damage event's amount.</summary>
        public float EventAmount() => Natives.GetEventDamage();

        /// <summary>The current damage event's attack type, 0 to 6.</summary>
        public int EventAttackType() => NumberIn(attackTypes, Natives.BlzGetEventAttackType());

        /// <summary>The current damage event's damage type, 0 to 26.</summary>
        public int EventDamageType() => NumberIn(damageTypes, Natives.BlzGetEventDamageType());

        /// <summary>The current damage event's weapon type, 0 to 23.</summary>
        public int EventWeaponType() => NumberIn(weaponTypes, Natives.BlzGetEventWeaponType());

        /// <summary>Whether the current damage event is an attack.</summary>
        public bool EventIsAttack() => Natives.BlzGetEventIsAttack();

        /// <summary>Sets the current damage event's amount.</summary>
        public void SetEventAmount(float amount) => Natives.BlzSetEventDamage(amount);

        /// <summary>Sets the current damage event's attack type.</summary>
        public void SetEventAttackType(int type) => Natives.BlzSetEventAttackType(attackTypes[type]);

        /// <summary>Sets the current damage event's damage type.</summary>
        public void SetEventDamageType(int type) => Natives.BlzSetEventDamageType(damageTypes[type]);

        /// <summary>Sets the current damage event's weapon type.</summary>
        public void SetEventWeaponType(int type) => Natives.BlzSetEventWeaponType(weaponTypes[type]);

        /// <summary>A unit's life.</summary>
        public float Life(unit target) => Natives.GetWidgetLife(target);

        /// <summary>Whether a unit is dead or gone.</summary>
        public bool IsDead(unit target) => Natives.IsUnitType(target, Natives.UNIT_TYPE_DEAD) || Natives.GetUnitTypeId(target) == 0;

        /// <summary>Whether a unit is magic immune.</summary>
        public bool IsMagicImmune(unit target) => Natives.IsUnitType(target, Natives.UNIT_TYPE_MAGIC_IMMUNE);

        /// <summary>Whether a unit counts as a melee attacker.</summary>
        public bool IsMelee(unit target) => Natives.IsUnitType(target, Natives.UNIT_TYPE_MELEE_ATTACKER);

        /// <summary>Whether a unit counts as a ranged attacker.</summary>
        public bool IsRanged(unit target) => Natives.IsUnitType(target, Natives.UNIT_TYPE_RANGED_ATTACKER);

        /// <summary>A unit's armor.</summary>
        public float Armor(unit target) => Natives.BlzGetUnitArmor(target);

        /// <summary>Sets a unit's armor.</summary>
        public void SetArmor(unit target, float armor) => Natives.BlzSetUnitArmor(target, armor);

        /// <summary>A unit's defense type.</summary>
        public int DefenseType(unit target) => Natives.BlzGetUnitIntegerField(target, Natives.UNIT_IF_DEFENSE_TYPE);

        /// <summary>Sets a unit's defense type.</summary>
        public void SetDefenseType(unit target, int type) => Natives.BlzSetUnitIntegerField(target, Natives.UNIT_IF_DEFENSE_TYPE, type);

        /// <summary>Deals damage from code.</summary>
        public void Deal(unit source, unit target, float amount, bool attack, bool ranged, int attackType, int damageType, int weaponType)
        {
            Natives.UnitDamageTarget(source, target, amount, attack, ranged, attackTypes[attackType], damageTypes[damageType], weaponTypes[weaponType]);
        }

        /// <summary>The number of a type in its list; 0 for one the list lacks.</summary>
        private static int NumberIn<T>(List<T> types, T type)
            where T : class
        {
            for (var i = 0; i < types.Count; i++)
            {
                if (types[i] == type)
                {
                    return i;
                }
            }

            return 0;
        }

        /// <summary>Runs the action after a 0 s timer; a call while it runs replaces the action.</summary>
        public void AfterZero(Action action)
        {
            var running = zeroAction != null;
            zeroAction = action;
            if (running)
            {
                return;
            }

            if (zero == null)
            {
                zero = Natives.CreateTimer();
            }

            Natives.TimerStart(zero, 0.0f, false, ZeroAction);
        }

        /// <summary>The 0 s timer's callback.</summary>
        private void ZeroAction()
        {
            var action = zeroAction;
            zeroAction = null;
            try
            {
                if (action != null)
                {
                    action();
                }
            }
            catch (Exception error)
            {
                Errors.Report("Moonwell.Damage", error);
            }
        }

        /// <summary>A trigger that runs the action on a player unit event for every slot; a failing action is reported.</summary>
        private static void Register(playerunitevent playerEvent, Action action)
        {
            var trigger = Natives.CreateTrigger();
            for (var slot = 0; slot < Natives.bj_MAX_PLAYER_SLOTS; slot++)
            {
                Natives.TriggerRegisterPlayerUnitEvent(trigger, Natives.Player(slot), playerEvent, null);
            }

            Natives.TriggerAddAction(trigger, () =>
            {
                try
                {
                    action();
                }
                catch (Exception error)
                {
                    Errors.Report("Moonwell.Damage", error);
                }
            });
        }
    }
}
