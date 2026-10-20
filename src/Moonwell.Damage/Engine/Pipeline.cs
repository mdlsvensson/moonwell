using System.Collections.Generic;
using Moonwell.Hosting;

namespace Moonwell
{
    /// <summary>Runs each hit through its stages: the package's state, and its actions on the game's damage events.</summary>
    internal static class Pipeline
    {
        /// <summary>How long a chain of hits dealt from reactions may be.</summary>
        internal const int MaxChain = 8;

        /// <summary>The world the package talks to; set by Install.</summary>
        internal static IDamageWorld World = null!;

        /// <summary>Whether the package listens for the game's damage events.</summary>
        private static bool listening;

        /// <summary>Whether a class implementing IDamageWatch has been seen.</summary>
        internal static bool Watched;

        /// <summary>The hits between their before-armor and after-armor events, innermost last.</summary>
        private static readonly List<Hit> open = new List<Hit>();

        /// <summary>Makes a world the package's world, clears its state and plugs it into the runtime.</summary>
        internal static void Install(IDamageWorld world)
        {
            World = world;
            listening = false;
            Watched = false;
            open.Clear();
            Reactions.Reset();
            Events.DamageListener = Listen;
            Hooks.Seen = Saw;
        }

        /// <summary>Starts listening for the game's damage events, once.</summary>
        internal static void Listen()
        {
            if (listening)
            {
                return;
            }

            listening = true;
            World.Listen(Damaging, Damaged);
        }

        /// <summary>A class seen for the first time: one implementing a damage interface makes the package listen.</summary>
        private static void Saw(object obj)
        {
            if (obj is IDamageWatch)
            {
                Watched = true;
            }

            if (obj is IDamageSource || obj is IDamageTarget || obj is IDamageWatch)
            {
                Listen();
            }
        }

        /// <summary>Whether anything can react to a hit between two units: a unit the runtime holds, or a map-wide reaction.</summary>
        private static bool Wanted(unit target, unit? source)
        {
            return Units.Find(target) != null || (source != null && Units.Find(source) != null) || Watched || Events.Wants(Hook.OnUnitDamaged);
        }

        /// <summary>Makes the hit of the current event, read from the game.</summary>
        private static Hit? Read()
        {
            var targetHandle = World.EventTarget();
            var sourceHandle = World.EventSource();
            var target = Unit.Of(targetHandle);
            if (target == null)
            {
                return null;
            }

            var source = sourceHandle == null ? null : Unit.Of(sourceHandle);
            var hit = new Hit(target, source, World.EventAmount(), World.EventIsAttack());
            hit.AttackTypeCode = World.EventAttackType();
            hit.DefenseTypeCode = World.DefenseType(targetHandle);
            hit.Fill((DamageType)World.EventDamageType(), (WeaponSound)World.EventWeaponType(), World.Armor(targetHandle));
            if (hit.IsAttack && sourceHandle != null)
            {
                var melee = World.IsMelee(sourceHandle);
                var ranged = World.IsRanged(sourceHandle);
                if (melee && ranged)
                {
                    ranged = hit.WeaponSound == WeaponSound.None;
                    melee = !ranged;
                }

                hit.IsMelee = melee;
                hit.IsRanged = ranged;
            }

            return hit;
        }

        /// <summary>The game's damaging event: the before-armor stage, then what it changed written back to the game.</summary>
        private static void Damaging()
        {
            if (!Wanted(World.EventTarget(), World.EventSource()))
            {
                return;
            }

            var hit = Read();
            if (hit == null)
            {
                return;
            }

            var amount = hit.Amount;
            var attackType = hit.AttackTypeCode;
            var damageType = hit.DamageType;
            var sound = hit.WeaponSound;
            var armor = hit.Armor;
            var defense = hit.DefenseTypeCode;
            hit.Stage = Stage.BeforeArmor;
            Reactions.Run(hit, Stage.BeforeArmor);
            hit.Stage = Stage.AfterArmor;

            if (hit.Amount != amount)
            {
                World.SetEventAmount(hit.Amount);
            }

            if (hit.AttackTypeCode != attackType)
            {
                World.SetEventAttackType(hit.AttackTypeCode);
            }

            if (hit.DamageType != damageType)
            {
                World.SetEventDamageType((int)hit.DamageType);
            }

            if (hit.WeaponSound != sound)
            {
                World.SetEventWeaponType((int)hit.WeaponSound);
            }

            var target = hit.Target.Handle;
            if (hit.Armor != armor)
            {
                PutArmor(hit, target);
            }

            if (hit.DefenseTypeCode != defense)
            {
                hit.DefenseChanged = true;
                hit.DefenseRestore = defense;
                World.SetDefenseType(target, hit.DefenseTypeCode);
            }

            open.Add(hit);
        }

        /// <summary>The game's damaged event: the target's own armor back, core's reactions, then the after-armor stage.</summary>
        private static void Damaged()
        {
            var targetHandle = World.EventTarget();
            Hit? hit = null;
            if (open.Count > 0 && open[open.Count - 1].Target.Handle == targetHandle)
            {
                hit = open[open.Count - 1];
                open.RemoveAt(open.Count - 1);
                PutBack(hit);
                hit.Amount = World.EventAmount();
            }
            else if (Wanted(targetHandle, World.EventSource()))
            {
                hit = Read();
            }

            if (hit == null)
            {
                return;
            }

            var amount = hit.Amount;
            hit.Stage = Stage.AfterArmor;
            hit.LifeBefore = World.Life(targetHandle);
            Inbound.Dispatch(hit);
            Reactions.Run(hit, Stage.AfterArmor);
            if (hit.Amount != amount)
            {
                World.SetEventAmount(hit.Amount);
            }
        }

        /// <summary>Puts the hit's armor on the target, whether the game's armor setter sets the armor with its bonuses or without them, and remembers what puts the target's own back.</summary>
        private static void PutArmor(Hit hit, unit target)
        {
            var before = World.Armor(target);
            World.SetArmor(target, hit.Armor);
            var bonus = World.Armor(target) - hit.Armor;
            if (bonus != 0f)
            {
                World.SetArmor(target, hit.Armor - bonus);
            }

            hit.ArmorChanged = true;
            hit.ArmorRestore = before - bonus;
        }

        /// <summary>Puts back the armor and defense type the hit put on its target.</summary>
        private static void PutBack(Hit hit)
        {
            var target = hit.Target.Handle;
            if (hit.ArmorChanged)
            {
                hit.ArmorChanged = false;
                World.SetArmor(target, hit.ArmorRestore);
            }

            if (hit.DefenseChanged)
            {
                hit.DefenseChanged = false;
                World.SetDefenseType(target, hit.DefenseRestore);
            }
        }
    }
}
