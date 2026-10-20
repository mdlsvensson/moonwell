using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>Adds several starting components in one call: <c>Components.Add(new Start(), new Rules());</c>.</summary>
    public static class ComponentLists
    {
        /// <summary>Adds the components to the end of the list, in the order given.</summary>
        public static void Add<T>(this List<T> list, T first, T second, params T[] more)
            where T : Component
        {
            list.Add(first);
            list.Add(second);
            for (var i = 0; i < more.Length; i++)
            {
                list.Add(more[i]);
            }
        }
    }
}
