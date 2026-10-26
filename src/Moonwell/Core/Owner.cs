using System;

namespace Moonwell
{
    /// <summary>An owned object that holds other owned objects, and ends them newest first when it ends.</summary>
    public abstract class Owner : Owned
    {
        /// <summary>The owner that running code makes new objects for; null outside Run.</summary>
        internal static Owner? current;

        // The instance fields are internal, not private, as a map's subclass may name a field of its own the same:
        // CSharp.lua gives a subclass's field a name of its own only when it sees the base's field of that name,
        // and it does not see a private one, so in Lua the two would be one field (PrivateFieldTests).

        /// <summary>The newest child, the first one to end.</summary>
        internal Owned? newest;

        /// <summary>Whether the children have ended; a child adopted from then on ends at once.</summary>
        internal bool childrenEnded;

        /// <summary>How many children are linked to this owner.</summary>
        internal int count;

        /// <summary>The owner running code makes new objects for: the one set by Run, else the root.</summary>
        [LocalSafe]
        public static Owner Current
        {
            get
            {
                return current ?? Runtime.Root;
            }
        }

        /// <summary>How many children this owner holds that have not ended.</summary>
        [LocalSafe]
        public int OwnedCount
        {
            get
            {
                return count;
            }
        }

        /// <summary>Runs an action with this owner as the current owner, and restores the previous one after, even when it throws.</summary>
        public void Run(Action action)
        {
            var previous = current;
            current = this;
            try
            {
                action();
            }
            finally
            {
                current = previous;
            }
        }

        /// <summary>Takes a child under this owner; a child adopted by an ended owner ends at once.</summary>
        internal void Adopt(Owned child)
        {
            Dev.Check(child.Owner == null && !child.HasEnded, "Owner.Adopt: the child already has an owner or has ended");
            child.Owner = this;
            if (HasEnded || childrenEnded)
            {
                child.End();
                return;
            }

            child.before = newest;
            if (newest != null)
            {
                newest.after = child;
            }

            newest = child;
            child.linked = true;
            count++;
        }

        /// <summary>Takes a child out of the list of children, so it does not end with this owner.</summary>
        internal void Unlink(Owned child)
        {
            if (!child.linked)
            {
                return;
            }

            if (child.after != null)
            {
                child.after.before = child.before;
            }
            else
            {
                newest = child.before;
            }

            if (child.before != null)
            {
                child.before.after = child.after;
            }

            child.before = null;
            child.after = null;
            child.linked = false;
            count--;
        }
    }
}
