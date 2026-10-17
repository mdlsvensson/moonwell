using System;

namespace Moonwell
{
    /// <summary>Something an owner holds, and ends when the owner ends or when it ends by itself.</summary>
    public abstract class Owned
    {
        /// <summary>The owner that adopted this object; kept after the object ends.</summary>
        public Owner? Owner { get; internal set; }

        /// <summary>Whether this object has ended.</summary>
        public bool HasEnded { get; private set; }

        /// <summary>The older sibling in the owner's list of children; null for the oldest.</summary>
        internal Owned? before;

        /// <summary>The newer sibling in the owner's list of children; null for the newest.</summary>
        internal Owned? after;

        /// <summary>Whether this object is in its owner's list of children.</summary>
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

            if (this is Owner owner)
            {
                while (owner.newest != null)
                {
                    var child = owner.newest;
                    owner.Unlink(child);
                    child.End();
                }
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
            if (linked && Owner != null)
            {
                Owner.Unlink(this);
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
