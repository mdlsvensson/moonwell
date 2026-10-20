using System;

namespace Moonwell.Hosting
{
    /// <summary>The game as the damage package sees it: the game's damage events, the units' life and armor, dealing damage, and a 0 s timer. Types are the game's numbers.</summary>
    public interface IDamageWorld
    {
        /// <summary>Starts delivering the game's damaging (before armor) and damaged (after armor) events.</summary>
        void Listen(Action damaging, Action damaged);

        /// <summary>The current damage event's source; null when it has none.</summary>
        unit? EventSource();

        /// <summary>The current damage event's target.</summary>
        unit EventTarget();

        /// <summary>The current damage event's amount.</summary>
        float EventAmount();

        /// <summary>The current damage event's attack type.</summary>
        int EventAttackType();

        /// <summary>The current damage event's damage type.</summary>
        int EventDamageType();

        /// <summary>The current damage event's weapon type.</summary>
        int EventWeaponType();

        /// <summary>Whether the current damage event is an attack.</summary>
        bool EventIsAttack();

        /// <summary>Sets the current damage event's amount.</summary>
        void SetEventAmount(float amount);

        /// <summary>Sets the current damage event's attack type.</summary>
        void SetEventAttackType(int type);

        /// <summary>Sets the current damage event's damage type.</summary>
        void SetEventDamageType(int type);

        /// <summary>Sets the current damage event's weapon type.</summary>
        void SetEventWeaponType(int type);

        /// <summary>A unit's life.</summary>
        float Life(unit target);

        /// <summary>Whether a unit is dead.</summary>
        bool IsDead(unit target);

        /// <summary>Whether a unit is magic immune.</summary>
        bool IsMagicImmune(unit target);

        /// <summary>Whether a unit counts as a melee attacker.</summary>
        bool IsMelee(unit target);

        /// <summary>Whether a unit counts as a ranged attacker.</summary>
        bool IsRanged(unit target);

        /// <summary>A unit's armor, as the game reads it.</summary>
        float Armor(unit target);

        /// <summary>Sets a unit's armor.</summary>
        void SetArmor(unit target, float armor);

        /// <summary>A unit's defense type.</summary>
        int DefenseType(unit target);

        /// <summary>Sets a unit's defense type.</summary>
        void SetDefenseType(unit target, int type);

        /// <summary>Deals damage from code.</summary>
        void Deal(unit source, unit target, float amount, bool attack, bool ranged, int attackType, int damageType, int weaponType);

        /// <summary>Runs the action after a 0 s timer; a call while the timer runs replaces the action.</summary>
        void AfterZero(Action action);
    }
}
