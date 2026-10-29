using System;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>A unit component that lasts for a duration, ticks at an interval, and stacks with the buffs of its class.</summary>
    public abstract class Buff : UnitComponent
    {
        /// <summary>The timer that ends the buff; a refresh ends it and starts a new one. Null when the buff has no expiry.</summary>
        internal Timer? expiry;

        /// <summary>The seconds the buff lasts; 0, the default, lasts until it is removed. A refresh gives the buff there the duration of the new apply, from the refresh on.</summary>
        [LocalSafe]
        public float Duration { get; protected set; } = 0.0f;

        /// <summary>The seconds between ticks; 0 for no ticks.</summary>
        [LocalSafe]
        public float Tick { get; protected set; } = 0.0f;

        /// <summary>What a second apply of the buff does when the unit already has one of its class.</summary>
        [LocalSafe]
        public Stacking Stacking { get; protected set; } = Stacking.Refresh;

        /// <summary>Whether the unit's death ends the buff, after the buff has seen it; true unless the class keeps it through death.</summary>
        [LocalSafe]
        public bool EndsOnDeath { get; protected set; } = true;

        /// <summary>The unit that applied the buff last; null when none did.</summary>
        [LocalSafe]
        public Unit? Source { get; internal set; }

        /// <summary>The seconds until the buff expires; 0 without an expiry.</summary>
        [LocalSafe]
        public float Remaining
        {
            get
            {
                return expiry != null && !expiry.HasEnded ? expiry.Remaining : 0.0f;
            }
        }

        /// <summary>Runs when the buff is applied, and again each time it is applied while it is on.</summary>
        public virtual void OnApply()
        {
        }

        /// <summary>Runs at each tick.</summary>
        public virtual void OnTick()
        {
        }

        /// <summary>Runs when the buff expires, before it ends.</summary>
        public virtual void OnExpire()
        {
        }

        /// <summary>Ends the buffs a death ends, newest first, of those that were on the unit as it died (Inbound takes them at the start of the death, and calls this after its reactions): a buff the reactions applied has not seen the death and stays.</summary>
        internal static void EndOnDeath(List<Buff> found)
        {
            for (var i = found.Count - 1; i >= 0; i--)
            {
                if (found[i].EndsOnDeath)
                {
                    found[i].End();
                }
            }
        }

        /// <summary>Starts a buff that was just added to its unit: it shows when it is the first of its class, starts its ticks and expiry, listens for deaths when a death ends it, and runs OnApply.</summary>
        internal void Begin()
        {
            if (EndsOnDeath)
            {
                Events.Need(Hook.OnDeath);
            }

            if (!HasOtherLive())
            {
                Show();
            }

            if (Tick > 0.0f)
            {
                OwnEvery(Tick, () => CallTick());
            }

            if (Duration > 0.0f)
            {
                StartExpiry();
            }

            CallApply();
        }

        /// <summary>Refreshes a buff already on its unit with a new apply of its class (decision 66): it takes the new buff's duration and the source, starts its expiry again for that duration (a duration of 0 lasts until it is removed), and runs OnApply again. Its other values, and what it has counted, stay its own; the new buff is dropped.</summary>
        internal void Refresh(Buff from, Unit? source)
        {
            Source = source;
            Duration = from.Duration;
            if (Duration > 0.0f)
            {
                StartExpiry();
            }
            else if (expiry != null)
            {
                expiry.End();
                expiry = null;
            }

            CallApply();
        }

        /// <summary>Runs after the buff's children have ended: it removes its display from the unit when it was the last buff of its class, and the unit is still in the game.</summary>
        internal override void Finish()
        {
            base.Finish();
            if (actor == null || Unit.IsGone || HasOtherLive())
            {
                return;
            }

            var carrier = ObjectTypes.CarrierOf(this);
            if (carrier == 0)
            {
                return;
            }

            Runtime.World.RemoveAbility(Unit.Handle, carrier);
            Runtime.World.RemoveAbility(Unit.Handle, ObjectTypes.BuffIdOf(this));
        }

        /// <summary>Whether another buff of this class is live on the unit.</summary>
        private bool HasOtherLive()
        {
            var found = Unit.GetAll<Buff>();
            var key = Names.ClassOf(this);
            for (var i = 0; i < found.Count; i++)
            {
                var other = found[i];
                if (other != this && !other.HasEnded && Names.ClassOf(other) == key)
                {
                    return true;
                }
            }

            return false;
        }

        /// <summary>Shows the buff on its unit through its carrier ability, when a display is registered for its class. The carrier is made permanent, as a morph (Metamorphosis, Bear Form) takes away an ability code added that is not, and its icon with it (seen 10-26).</summary>
        private void Show()
        {
            var carrier = ObjectTypes.CarrierOf(this);
            if (carrier == 0)
            {
                return;
            }

            Runtime.World.AddAbility(Unit.Handle, carrier);
            Runtime.World.HideAbility(Unit.Handle, carrier);
            Runtime.World.MakeAbilityPermanent(Unit.Handle, carrier);
        }

        /// <summary>Ends the expiry timer the buff has, if any, and starts a new one for its duration.</summary>
        private void StartExpiry()
        {
            if (expiry != null)
            {
                expiry.End();
            }

            Timer? made = null;
            made = OwnAfter(Duration, () => Expire(made));
            expiry = made;
        }

        /// <summary>Runs OnExpire, then ends the buff unless OnExpire ended it or applied it again.</summary>
        private void Expire(Timer? made)
        {
            if (HasEnded)
            {
                return;
            }

            CallExpire();
            if (!HasEnded && expiry == made)
            {
                End();
            }
        }

        /// <summary>Runs OnApply as a reaction of the buff.</summary>
        private void CallApply()
        {
            if (HasEnded)
            {
                return;
            }

            try
            {
                OnApply();
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(this) + ".OnApply", error);
            }
        }

        /// <summary>Runs OnTick as a reaction of the buff.</summary>
        private void CallTick()
        {
            if (HasEnded)
            {
                return;
            }

            try
            {
                OnTick();
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(this) + ".OnTick", error);
            }
        }

        /// <summary>Runs OnExpire as a reaction of the buff.</summary>
        private void CallExpire()
        {
            if (HasEnded)
            {
                return;
            }

            try
            {
                OnExpire();
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(this) + ".OnExpire", error);
            }
        }
    }
}
