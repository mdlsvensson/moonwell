using System;

namespace Moonwell
{
    /// <summary>Something that holds what is made in it, and ends it, newest first, when it ends.</summary>
    public abstract class Scope : Scoped
    {
        // The instance fields are internal, not private, as a map's subclass may name a field of its own the same:
        // CSharp.lua gives a subclass's field a name of its own only when it sees the base's field of that name,
        // and it does not see a private one, so in Lua the two would be one field (PrivateFieldTests).

        /// <summary>The newest child, the first one to end.</summary>
        internal Scoped? newest;

        /// <summary>Whether the children have ended; a child adopted from then on ends at once.</summary>
        internal bool childrenEnded;

        /// <summary>How many children are linked to this scope.</summary>
        internal int count;

        /// <summary>How many children this scope holds that have not ended.</summary>
        [LocalSafe]
        public int ChildCount
        {
            get
            {
                return count;
            }
        }

        /// <summary>Runs an action once after the seconds, as a timer in this scope: it ends with the scope if the scope ends first.</summary>
        public Timer After(float seconds, Action action)
        {
            Dev.Check(seconds >= 0.0f, Names.NameOf(this) + ".After: seconds must be 0 or more");
            return Timer.Start(new Timer(Runtime.World.Now(), seconds, false, action), this);
        }

        /// <summary>
        /// Runs an action at every interval of the seconds, as a timer in this scope, until the scope ends. Seconds of 0 or less
        /// are reported and refused in every build, since such a timer would freeze the game: the timer given back has ended and
        /// never runs (decision 74).
        /// </summary>
        public Timer Every(float seconds, Action action)
        {
            // Not more than 0, so NaN is refused too.
            if (!(seconds > 0.0f))
            {
                return Timer.Refuse(Names.NameOf(this) + ".Every", seconds, action);
            }

            return Timer.Start(new Timer(Runtime.World.Now(), seconds, true, action), this);
        }

        /// <summary>Runs an action once after the seconds, as a timer of Moonwell's own work in this scope, which the never-ran report skips.</summary>
        internal Timer OwnAfter(float seconds, Action action)
        {
            var timer = new Timer(Runtime.World.Now(), seconds, false, action);
            timer.own = true;
            return Timer.Start(timer, this);
        }

        /// <summary>Runs an action at every interval of the seconds, more than 0, as a timer of Moonwell's own work in this scope, which the never-ran report skips.</summary>
        internal Timer OwnEvery(float seconds, Action action)
        {
            var timer = new Timer(Runtime.World.Now(), seconds, true, action);
            timer.own = true;
            return Timer.Start(timer, this);
        }

        /// <summary>Takes a child under this scope; a child adopted by an ended scope ends at once. A child that already has a parent, or has ended, is left as it is, and the error is reported, in every build: linked twice, it would break the scopes' lists (decision 74).</summary>
        internal void Adopt(Scoped child)
        {
            if (child.Parent != null || child.HasEnded)
            {
                Errors.Refuse(Names.NameOf(this) + ".Adopt", Names.NameOf(child) + " already has a parent or has ended");
                return;
            }

            child.Parent = this;
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

        /// <summary>Takes a child out of the list of children, so it does not end with this scope.</summary>
        internal void Unlink(Scoped child)
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
