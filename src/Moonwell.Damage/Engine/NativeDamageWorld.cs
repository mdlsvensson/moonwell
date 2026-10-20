using System;
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

        /// <summary>Registers both damage events for every player slot.</summary>
        public void Listen(Action damaging, Action damaged)
        {
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
        public int EventAttackType()
        {
            var type = Natives.BlzGetEventAttackType();
            for (var i = 0; i <= 6; i++)
            {
                if (type == Natives.ConvertAttackType(i))
                {
                    return i;
                }
            }

            return 0;
        }

        /// <summary>The current damage event's damage type, 0 to 26.</summary>
        public int EventDamageType()
        {
            var type = Natives.BlzGetEventDamageType();
            for (var i = 0; i <= 26; i++)
            {
                if (type == Natives.ConvertDamageType(i))
                {
                    return i;
                }
            }

            return 0;
        }

        /// <summary>The current damage event's weapon type, 0 to 23.</summary>
        public int EventWeaponType()
        {
            var type = Natives.BlzGetEventWeaponType();
            for (var i = 0; i <= 23; i++)
            {
                if (type == Natives.ConvertWeaponType(i))
                {
                    return i;
                }
            }

            return 0;
        }

        /// <summary>Whether the current damage event is an attack.</summary>
        public bool EventIsAttack() => Natives.BlzGetEventIsAttack();

        /// <summary>Sets the current damage event's amount.</summary>
        public void SetEventAmount(float amount) => Natives.BlzSetEventDamage(amount);

        /// <summary>Sets the current damage event's attack type.</summary>
        public void SetEventAttackType(int type) => Natives.BlzSetEventAttackType(Natives.ConvertAttackType(type));

        /// <summary>Sets the current damage event's damage type.</summary>
        public void SetEventDamageType(int type) => Natives.BlzSetEventDamageType(Natives.ConvertDamageType(type));

        /// <summary>Sets the current damage event's weapon type.</summary>
        public void SetEventWeaponType(int type) => Natives.BlzSetEventWeaponType(Natives.ConvertWeaponType(type));

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
            Natives.UnitDamageTarget(source, target, amount, attack, ranged, Natives.ConvertAttackType(attackType), Natives.ConvertDamageType(damageType), Natives.ConvertWeaponType(weaponType));
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

            Natives.TimerStart(zero, 0f, false, ZeroAction);
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
