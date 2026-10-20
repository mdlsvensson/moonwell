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

        /// <summary>The hits that finished their after-armor stage since the 0 s timer last ran, in order: they land when it runs.</summary>
        private static List<Hit> moment = new List<Hit>();

        /// <summary>The deal from code waiting for its before-armor event; null when none.</summary>
        private static PendingDeal? pending;

        /// <summary>Makes a world the package's world, clears its state and plugs it into the runtime.</summary>
        internal static void Install(IDamageWorld world)
        {
            World = world;
            listening = false;
            Watched = false;
            open.Clear();
            moment = new List<Hit>();
            pending = null;
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

        /// <summary>Deals damage from code: the hit its before-armor event makes is marked as code's and returned. A deal from a reaction continues the reacting hit's chain; one that would make the chain longer than MaxChain throws, which the reaction reports.</summary>
        internal static Hit? Deal(Unit source, Unit target, float amount, int attackType, DamageKind type, bool isAttack, bool ranged, WeaponSound sound, object? tag)
        {
            var running = Reactions.Running;
            var chain = running.Count == 0 ? 0 : running[running.Count - 1].Chain + 1;
            if (chain >= MaxChain)
            {
                throw new MoonwellException(Names.NameOf(source) + " to " + Names.NameOf(target) + ": not dealt, a chain of " + MaxChain + " hits dealt from reactions ends here");
            }

            Listen();
            var previous = pending;
            var deal = new PendingDeal(source.Handle, target.Handle, isAttack, ranged, tag, chain);
            pending = deal;
            try
            {
                World.Deal(source.Handle, target.Handle, amount, isAttack, ranged, attackType, (int)type, (int)sound);
            }
            finally
            {
                pending = previous;
            }

            return deal.Hit;
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
            var deal = pending != null && pending.Hit == null && pending.Target == targetHandle && pending.Source == sourceHandle ? pending : null;
            var hit = new Hit(target, source, World.EventAmount(), deal != null ? deal.IsAttack : World.EventIsAttack());
            if (deal != null)
            {
                deal.Hit = hit;
                hit.IsCode = true;
                hit.Tag = deal.Tag;
                hit.Chain = deal.Chain;
            }

            hit.AttackTypeCode = World.EventAttackType();
            hit.DefenseTypeCode = World.DefenseType(targetHandle);
            hit.Fill((DamageKind)World.EventDamageType(), (WeaponSound)World.EventWeaponType(), World.Armor(targetHandle));
            if (deal != null)
            {
                hit.IsRanged = deal.IsAttack && deal.Ranged;
                hit.IsMelee = deal.IsAttack && !deal.Ranged;
            }
            else if (hit.IsAttack && sourceHandle != null)
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
            if (pending == null && !Wanted(World.EventTarget(), World.EventSource()))
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
            var damageType = hit.DamageKind;
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

            if (hit.DamageKind != damageType)
            {
                World.SetEventDamageType((int)hit.DamageKind);
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

            // Spell immunity: the game will not send this hit's after-armor event, so it ends here.
            if (Immunity.Stops(World.IsMagicImmune(target), hit.AttackTypeCode, (int)hit.DamageKind))
            {
                PutBack(hit);
                return;
            }

            open.Add(hit);
            World.AfterZero(Settle);
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
            if (hit.LifeBefore - hit.Amount <= 0.405f)
            {
                hit.Stage = Stage.Lethal;
                Reactions.Run(hit, Stage.Lethal);
            }

            if (hit.Amount != amount)
            {
                World.SetEventAmount(hit.Amount);
            }

            moment.Add(hit);
            World.AfterZero(Settle);
        }

        /// <summary>The 0 s timer: hits still waiting for their after-armor event end without it; the moment's hits land, in order; then each source's burst.</summary>
        private static void Settle()
        {
            while (open.Count > 0)
            {
                PutBack(open[open.Count - 1]);
                open.RemoveAt(open.Count - 1);
            }

            var landed = moment;
            moment = new List<Hit>();
            var bursts = new List<Burst>();
            for (var i = 0; i < landed.Count; i++)
            {
                var hit = landed[i];
                hit.Stage = Stage.Landed;
                hit.Killed = World.IsDead(hit.Target.Handle);
                Reactions.Run(hit, Stage.Landed);
                if (hit.Source != null)
                {
                    AddToBurst(bursts, hit);
                }
            }

            for (var i = 0; i < bursts.Count; i++)
            {
                Reactions.RunBurst(bursts[i]);
            }
        }

        /// <summary>Adds a hit to its source's burst, made when it is the source's first; counts a target the burst has not hit yet.</summary>
        private static void AddToBurst(List<Burst> bursts, Hit hit)
        {
            Burst? burst = null;
            for (var i = 0; i < bursts.Count; i++)
            {
                if (bursts[i].Source == hit.Source)
                {
                    burst = bursts[i];
                }
            }

            if (burst == null)
            {
                burst = new Burst(hit.Source!);
                bursts.Add(burst);
            }

            var known = false;
            for (var i = 0; i < burst.Hits.Count; i++)
            {
                known = known || burst.Hits[i].Target == hit.Target;
            }

            if (!known)
            {
                burst.Targets++;
            }

            burst.Hits.Add(hit);
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

    /// <summary>A deal from code waiting for the game's before-armor event it makes.</summary>
    internal sealed class PendingDeal
    {
        /// <summary>Makes the record of a deal.</summary>
        internal PendingDeal(unit source, unit target, bool isAttack, bool ranged, object? tag, int chain)
        {
            Source = source;
            Target = target;
            IsAttack = isAttack;
            Ranged = ranged;
            Tag = tag;
            Chain = chain;
        }

        /// <summary>The dealing unit.</summary>
        internal readonly unit Source;

        /// <summary>The unit dealt to.</summary>
        internal readonly unit Target;

        /// <summary>Whether reactions see the hit as an attack.</summary>
        internal readonly bool IsAttack;

        /// <summary>Whether an attack is ranged.</summary>
        internal readonly bool Ranged;

        /// <summary>The map's tag.</summary>
        internal readonly object? Tag;

        /// <summary>The hit's place in its chain.</summary>
        internal readonly int Chain;

        /// <summary>The hit the deal made; null until its before-armor event.</summary>
        internal Hit? Hit;
    }
}
