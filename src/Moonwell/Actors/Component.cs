using System;

namespace Moonwell
{
    /// <summary>A part of an actor: it is owned by the actor, and ends when it is removed or when the actor ends.</summary>
    public abstract class Component : Owner
    {
        /// <summary>The actor the component is on; null until it is added.</summary>
        internal Actor? actor;

        /// <summary>Whether the component has been added to its actor, so that its OnRemoved runs when it ends.</summary>
        internal bool added;

        /// <summary>The bit mask of the reactions this class overrides, worked out when the component is added.</summary>
        internal int Mask;

        /// <summary>The actor the component is on; null until it is added.</summary>
        public Actor Actor => actor!;

        /// <summary>The reactions this kind of component declares.</summary>
        internal abstract HookTable Table { get; }

        /// <summary>Runs when the component is added to its actor.</summary>
        protected internal virtual void OnAdded()
        {
        }

        /// <summary>Runs when the component is removed or its actor ends, before it leaves its actor.</summary>
        protected internal virtual void OnRemoved()
        {
        }

        /// <summary>Runs OnRemoved, then takes the component off its actor; a component never added does nothing.</summary>
        protected sealed override void OnEnd()
        {
            if (!added)
            {
                return;
            }

            var previous = Owner.current;
            Owner.current = this;
            try
            {
                OnRemoved();
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(this) + ".OnRemoved", error);
            }

            Owner.current = previous;
            actor!.Detach(this);
        }
    }
}
