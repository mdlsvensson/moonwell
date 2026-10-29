using System;

namespace Moonwell
{
    /// <summary>Something a scope holds, and ends when the scope ends or when it ends by itself.</summary>
    public abstract class Scoped
    {
        /// <summary>The scope this object was made in, which ends it when it ends; kept after the object ends.</summary>
        [LocalSafe]
        public Scope? Parent { get; internal set; }

        /// <summary>Whether this object has ended: false inside its OnEnd, OnRemoved and OnLeave, true once its End or removal returns.</summary>
        [LocalSafe]
        public bool HasEnded { get; private set; }

        /// <summary>The older sibling in its parent's list of children; null for the oldest.</summary>
        internal Scoped? before;

        /// <summary>The newer sibling in its parent's list of children; null for the newest.</summary>
        internal Scoped? after;

        /// <summary>Whether this object is in its parent's list of children.</summary>
        internal bool linked;

        /// <summary>Whether this object is ending; a second End while it runs does nothing.</summary>
        internal bool ending;

        /// <summary>Ends this object once: its own OnEnd, its children, then Finish.</summary>
        public void End()
        {
            if (ending || HasEnded)
            {
                return;
            }

            ending = true;
            try
            {
                OnEnd();
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(this) + ".OnEnd", error);
            }

            if (this is Scope scope)
            {
                while (scope.newest != null)
                {
                    var child = scope.newest;
                    scope.Unlink(child);
                    child.End();
                }

                scope.childrenEnded = true;
            }

            try
            {
                Finish();
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(this) + ".Finish", error);
            }

            HasEnded = true;
            if (linked && Parent != null)
            {
                Parent.Unlink(this);
            }
        }

        /// <summary>Runs when the object ends, before its children end.</summary>
        protected virtual void OnEnd()
        {
        }

        /// <summary>Runs after the children have ended; empty here.</summary>
        internal virtual void Finish()
        {
        }
    }
}
