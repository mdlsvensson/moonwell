using System;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>An owner that holds components, in the order they were added, and ends them when it ends.</summary>
    public abstract class Actor : Scope
    {
        // The instance fields are internal, not private, as a map's subclass may name a field of its own the same (see Scope).

        /// <summary>The components in add order; a removed one is null until the list is compacted.</summary>
        internal List<Component?> components = new List<Component?>();

        /// <summary>How many dispatches are running; the list is compacted only when none is.</summary>
        internal int dispatching;

        /// <summary>Whether the list holds a null left by a removal that is not compacted yet.</summary>
        internal bool holes;

        /// <summary>How many components are on this actor.</summary>
        [LocalSafe]
        public int ComponentCount
        {
            get
            {
                var count = 0;
                for (var i = 0; i < components.Count; i++)
                {
                    if (components[i] != null)
                    {
                        count++;
                    }
                }

                return count;
            }
        }

        /// <summary>The first component of a class in add order, or null when there is none.</summary>
        [LocalSafe]
        public T? Get<T>() where T : Component
        {
            for (var i = 0; i < components.Count; i++)
            {
                if (components[i] is T found)
                {
                    return found;
                }
            }

            return null;
        }

        /// <summary>Every component of a class, in add order.</summary>
        [LocalSafe]
        public List<T> GetAll<T>() where T : Component
        {
            var found = new List<T>();
            for (var i = 0; i < components.Count; i++)
            {
                if (components[i] is T match)
                {
                    found.Add(match);
                }
            }

            return found;
        }

        /// <summary>Whether a component of a class is on this actor.</summary>
        [LocalSafe]
        public bool Has<T>() where T : Component
        {
            return Get<T>() != null;
        }

        /// <summary>Removes a component from this actor: it ends, which runs its OnRemoved. A component that is not on this actor is left as it is, and the error is reported, in every build.</summary>
        public void Remove(Component component)
        {
            if (component.actor != this)
            {
                Errors.Refuse(Names.NameOf(this) + ".Remove", Names.NameOf(component) + " is not on this actor");
                return;
            }

            component.End();
        }

        /// <summary>Removes the first component of a class, in add order: it ends, which runs its OnRemoved. Returns whether there was one.</summary>
        public bool Remove<T>() where T : Component
        {
            var found = Get<T>();
            if (found == null)
            {
                return false;
            }

            Remove(found);
            return true;
        }

        /// <summary>Removes every component of a class and returns how many there were.</summary>
        public int RemoveAll<T>() where T : Component
        {
            var found = GetAll<T>();
            for (var i = 0; i < found.Count; i++)
            {
                Remove(found[i]);
            }

            return found.Count;
        }

        /// <summary>Whether a component may not be added to this actor, as it is on an actor already or has ended: it is then reported, in every build, as an error of the action, and the caller leaves it as it is (decision 74).</summary>
        internal bool Refuses(Component component, string action)
        {
            if (component.actor == null && !component.HasEnded)
            {
                return false;
            }

            Errors.Refuse(Names.NameOf(this) + "." + action, Names.NameOf(component) + " is already on an actor or has ended");
            return true;
        }

        /// <summary>Adds a component to this actor, runs its OnAdded, and returns it; on an actor that has ended it ends at once. A component already on an actor, or one that has ended, is left as it is, and the error is reported, in every build.</summary>
        protected TComponent AddComponent<TComponent>(TComponent component) where TComponent : Component
        {
            if (Refuses(component, "Add"))
            {
                return component;
            }

            component.actor = this;
            Adopt(component);
            if (component.HasEnded)
            {
                return component;
            }

            components.Add(component);
            component.Mask = Hooks.MaskOf(component, component.Table);
            Events.Need(component.Mask);
            component.added = true;
            var previous = Scope.current;
            Scope.current = component;
            try
            {
                component.CallAdded();
            }
            catch (Exception error)
            {
                Errors.Report(Names.NameOf(component) + ".OnAdded", error);
            }

            Scope.current = previous;
            return component;
        }

        /// <summary>Starts a dispatch and returns how many components there are now, so one added during it runs from the next dispatch.</summary>
        internal int BeginDispatch()
        {
            dispatching++;
            return components.Count;
        }

        /// <summary>The component at an index in add order, or null once it has been removed.</summary>
        internal Component? ComponentAt(int index)
        {
            return components[index];
        }

        /// <summary>Ends a dispatch, and compacts the list when no dispatch is running any more.</summary>
        internal void EndDispatch()
        {
            dispatching--;
            Compact();
        }

        /// <summary>Takes a component off the list: its slot is left null while a dispatch runs, and compacted at once otherwise.</summary>
        internal void Detach(Component component)
        {
            for (var i = 0; i < components.Count; i++)
            {
                if (components[i] == component)
                {
                    components[i] = null;
                    holes = true;
                    break;
                }
            }

            Compact();
        }

        /// <summary>Removes the nulls from the list, unless a dispatch is running.</summary>
        private void Compact()
        {
            if (dispatching > 0 || !holes)
            {
                return;
            }

            holes = false;
            var kept = new List<Component?>();
            for (var i = 0; i < components.Count; i++)
            {
                if (components[i] != null)
                {
                    kept.Add(components[i]);
                }
            }

            components = kept;
        }
    }
}
