using System;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>The object ids of the game's types, and the class each registered unit type is made as.</summary>
    public static class ObjectTypes
    {
        /// <summary>The function that makes the object of each registered unit type, keyed by its id; a lookup only.</summary>
        private static Dictionary<int, Func<Unit>> makers = new Dictionary<int, Func<Unit>>();

        /// <summary>The id each registered class is registered with, keyed by the class; a lookup only.</summary>
        private static Dictionary<Type, int> ids = new Dictionary<Type, int>();

        /// <summary>Registers a unit type with its id and the function that makes its object, and listens for the events its class needs.</summary>
        public static void Unit<T>(int id, Func<T> make)
            where T : Moonwell.Unit
        {
            if (makers.ContainsKey(id))
            {
                throw new MoonwellException("ObjectTypes.Unit: " + id + " is registered twice");
            }

            ids[typeof(T)] = id;
            makers[id] = make;
            Events.Need(Hooks.MaskOf(make(), Moonwell.Unit.UnitHooks));
        }

        /// <summary>The object id a class is registered with; throws when the class is not registered.</summary>
        public static int IdOf<T>()
        {
            if (!ids.TryGetValue(typeof(T), out var id))
            {
                throw new MoonwellException(typeof(T).Name + " has no object id: it is not registered");
            }

            return id;
        }

        /// <summary>The object id made from four characters, the first one the highest byte.</summary>
        public static int FourCC(string code)
        {
            return ((Code(code, 0) * 256 + Code(code, 1)) * 256 + Code(code, 2)) * 256 + Code(code, 3);
        }

        /// <summary>The function that makes the object of a registered unit type; null for an id that is not registered.</summary>
        internal static Func<Unit>? UnitMaker(int id)
        {
            if (makers.TryGetValue(id, out var maker))
            {
                return maker;
            }

            return null;
        }

        /// <summary>Forgets every registered type; Runtime.Reset calls it.</summary>
        internal static void Reset()
        {
            makers = new Dictionary<int, Func<Unit>>();
            ids = new Dictionary<Type, int>();
        }

        /// <summary>The character code at an index of a string.</summary>
        private static int Code(string code, int index)
        {
            return (int)code[index];
        }
    }
}
