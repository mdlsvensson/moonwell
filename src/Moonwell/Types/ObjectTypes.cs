using System;
using System.Collections.Generic;

namespace Moonwell
{
    /// <summary>The object ids of the game's types, and the class each registered unit type is made as.</summary>
    public static class ObjectTypes
    {
        /// <summary>The function that makes the object of each registered unit type, keyed by its id; a lookup only.</summary>
        private static Dictionary<int, Func<Unit>> makers = new Dictionary<int, Func<Unit>>();

        /// <summary>The function that makes the object of each registered item type, keyed by its id; a lookup only.</summary>
        private static Dictionary<int, Func<Item>> itemMakers = new Dictionary<int, Func<Item>>();

        /// <summary>The function that makes the object of each registered destructable type, keyed by its id; a lookup only.</summary>
        private static Dictionary<int, Func<Destructable>> destructableMakers = new Dictionary<int, Func<Destructable>>();

        /// <summary>The function that makes the object of each registered ability type, keyed by its id; a lookup only.</summary>
        private static Dictionary<int, Func<Moonwell.Ability>> abilityMakers = new Dictionary<int, Func<Moonwell.Ability>>();

        /// <summary>The id each registered class is registered with, keyed by the class; a lookup only.</summary>
        private static Dictionary<Type, int> ids = new Dictionary<Type, int>();

        /// <summary>The game buff each registered buff class shows, keyed by the class; a lookup only.</summary>
        private static Dictionary<object, int> buffIds = new Dictionary<object, int>();

        /// <summary>The hidden ability that carries each registered buff class on its unit, keyed by the class; a lookup only.</summary>
        private static Dictionary<object, int> carriers = new Dictionary<object, int>();

        /// <summary>The default object of each unit type, keyed by its id; made on first use.</summary>
        private static Dictionary<int, Unit>? unitDefaults;

        /// <summary>The default object of each item type, keyed by its id; made on first use.</summary>
        private static Dictionary<int, Item>? itemDefaults;

        /// <summary>The default object of each unit or item class that is not registered, keyed by the class; made on first use.</summary>
        private static Dictionary<Type, Widget>? classDefaults;

        /// <summary>The classes and type ids whose default object is being made, so that one read while its constructor runs fails rather than making it again; made on first use.</summary>
        private static HashSet<object>? making;

        /// <summary>While Registering runs its register step, the registrations' listens, which run their classes' constructors once every maker is registered; null otherwise, when a registration listens at once.</summary>
        private static List<Action>? waiting;

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
            Listen(() => Events.Need(Hooks.MaskOf(make(), Moonwell.Unit.UnitHooks)));
        }

        /// <summary>Registers an item type with its id and the function that makes its object, and listens for the events its class needs.</summary>
        public static void Item<T>(int id, Func<T> make)
            where T : Moonwell.Item
        {
            if (itemMakers.ContainsKey(id))
            {
                throw new MoonwellException("ObjectTypes.Item: " + id + " is registered twice");
            }

            ids[typeof(T)] = id;
            itemMakers[id] = make;
            Listen(() => Events.Need(Hooks.MaskOf(make(), Moonwell.Item.ItemHooks)));
        }

        /// <summary>Registers a destructable type with its id and the function that makes its object, and listens for the events its class needs.</summary>
        public static void Destructable<T>(int id, Func<T> make)
            where T : Moonwell.Destructable
        {
            if (destructableMakers.ContainsKey(id))
            {
                throw new MoonwellException("ObjectTypes.Destructable: " + id + " is registered twice");
            }

            ids[typeof(T)] = id;
            destructableMakers[id] = make;
            Listen(() => Events.Need(Hooks.MaskOf(make(), Moonwell.Destructable.DestructableHooks)));
        }

        /// <summary>Registers an ability type with its id and the function that makes its object, and listens for the events its class needs.</summary>
        public static void Ability<T>(int id, Func<T> make)
            where T : Moonwell.Ability
        {
            if (abilityMakers.ContainsKey(id))
            {
                throw new MoonwellException("ObjectTypes.Ability: " + id + " is registered twice");
            }

            ids[typeof(T)] = id;
            abilityMakers[id] = make;
            Listen(() => Events.Need(Hooks.MaskOf(make(), Moonwell.Ability.AbilityHooks)));
        }

        /// <summary>
        /// Runs a register step, then the constructors its registrations run to find the events their classes need: every maker is
        /// registered before any constructor runs, so that a constructor that reads the default object of a class registered after
        /// its own (<c>Unit.Default&lt;Lancer&gt;()</c>) gets that class's, made by its maker. Runtime.Install runs the map's registry with it.
        /// </summary>
        internal static void Registering(Action register)
        {
            var listens = new List<Action>();
            waiting = listens;
            try
            {
                register();
            }
            finally
            {
                waiting = null;
            }

            for (var i = 0; i < listens.Count; i++)
            {
                listens[i]();
            }
        }

        /// <summary>Runs a registration's listen, which makes an object of its class: at once, or once the register step Registering runs has registered every maker.</summary>
        private static void Listen(Action listen)
        {
            if (waiting != null)
            {
                waiting.Add(listen);
            }
            else
            {
                listen();
            }
        }

        /// <summary>Registers a class with its object id and no maker, for kinds such as doodads, upgrades and buffs; throws when the class is registered twice.</summary>
        public static void Id<T>(int id)
            where T : class
        {
            if (ids.ContainsKey(typeof(T)))
            {
                throw new MoonwellException("ObjectTypes.Id: " + typeof(T).Name + " is registered twice");
            }

            ids[typeof(T)] = id;
        }

        /// <summary>Registers a buff class with the game buff it shows and the hidden ability that carries it on its unit; a buff of the class shows only with this.</summary>
        public static void BuffDisplay<T>(int buffId, int carrierId)
            where T : Buff
        {
            var key = Names.ClassOfType(typeof(T));
            if (carriers.ContainsKey(key))
            {
                throw new MoonwellException("ObjectTypes.BuffDisplay: " + typeof(T).Name + " is registered twice");
            }

            buffIds[key] = buffId;
            carriers[key] = carrierId;
        }

        /// <summary>The object id a class is registered with; throws when the class is not registered.</summary>
        [LocalSafe]
        public static int IdOf<T>()
        {
            if (!ids.TryGetValue(typeof(T), out var id))
            {
                throw new MoonwellException(typeof(T).Name + " has no object id: it is not registered");
            }

            return id;
        }

        /// <summary>The object id the class of an object is registered with; 0 when it is not registered.</summary>
        internal static int IdOfClass(object obj)
        {
            return ids.TryGetValue(Names.TypeOf(obj), out var id) ? id : 0;
        }

        /// <summary>The default object of a class: the one of the type it is registered with, else one of its own, made with its constructor without parameters.</summary>
        internal static Widget DefaultOf<T>()
            where T : Widget, new()
        {
            var kind = typeof(T);
            var unitKind = typeof(Unit).IsAssignableFrom(kind);
            if (!unitKind && !typeof(Item).IsAssignableFrom(kind))
            {
                throw new MoonwellException(kind.Name + " has no default object: only units and items have one");
            }

            if (ids.TryGetValue(kind, out var id))
            {
                return unitKind ? DefaultUnit(id, false, kind) : DefaultItem(id, kind);
            }

            if (classDefaults != null && classDefaults.TryGetValue(kind, out var known))
            {
                return known;
            }

            return KeepClassDefault(kind, Make(kind, () => Another(kind)));
        }

        /// <summary>The default object of a unit's type: the one of its type id, or of its class when the class has no id and is not registered (see ClassDefault).</summary>
        internal static Unit DefaultOf(Unit unit, int typeId)
        {
            if (typeId != 0)
            {
                return DefaultUnit(typeId, unit is Hero, null);
            }

            return (Unit)ClassDefault(unit);
        }

        /// <summary>The default object of an item's type: the one of its type id, or of its class when the class has no id and is not registered.</summary>
        internal static Item DefaultOf(Item item, int typeId)
        {
            if (typeId != 0)
            {
                return DefaultItem(typeId, null);
            }

            return (Item)ClassDefault(item);
        }

        /// <summary>The default object of the class of an object with no type, made once with the class's constructor, as DefaultOf makes it: the same whichever is asked first.</summary>
        private static Widget ClassDefault(Widget widget)
        {
            var kind = Names.TypeOf(widget);
            if (classDefaults != null && classDefaults.TryGetValue(kind, out var known))
            {
                return known;
            }

            return KeepClassDefault(kind, Make(kind, () => Another(kind)));
        }

        /// <summary>Keeps a class's default object, gone, and gives it.</summary>
        private static Widget KeepClassDefault(Type kind, Widget made)
        {
            if (classDefaults == null)
            {
                classDefaults = new Dictionary<Type, Widget>();
            }

            made.IsGone = true;
            classDefaults[kind] = made;
            return made;
        }

        /// <summary>A new object of a unit or item class, made with its constructor without parameters; an exception the constructor throws comes out as it is (new T() wraps it on .NET). In Lua a type's table holds the class at [1], which makes an object when called.</summary>
        /// @CSharpLua.Template = "{0}[1]()"
        private static Widget Another(Type kind)
        {
            return Construct(kind);
        }

        /// <summary>Another on .NET; CSharp.lua does not compile it.</summary>
        /// @CSharpLua.Ignore
        private static Widget Construct(Type kind)
        {
            return (Widget)Activator.CreateInstance(kind, System.Reflection.BindingFlags.Instance | System.Reflection.BindingFlags.Public | System.Reflection.BindingFlags.NonPublic | System.Reflection.BindingFlags.DoNotWrapExceptions, null, null, null)!;
        }

        /// <summary>Makes the default object of a class or a registered type id (the key) with a function; throws, naming the class, when it is already being made: its constructor read it, so making it again would never end.</summary>
        private static T Make<T>(object key, Func<T> make)
            where T : Widget
        {
            if (making == null)
            {
                making = new HashSet<object>();
            }

            if (!making.Add(key))
            {
                throw new MoonwellException(ClassNameOf(key) + "'s default object is read while its constructor runs; read the field itself");
            }

            try
            {
                return make();
            }
            finally
            {
                making.Remove(key);
            }
        }

        /// <summary>The class a type id's default object is made as before its maker is registered: the most derived of the class asked for and the classes registered with the id that inherit it, so that a class that replaces a standard one, whose maker is the id's, is made rather than the standard one.</summary>
        private static Type ClassOfId(int typeId, Type asked)
        {
            var chosen = asked;
            foreach (var registered in ids)
            {
                if (registered.Value == typeId && registered.Key != chosen && chosen.IsAssignableFrom(registered.Key))
                {
                    chosen = registered.Key;
                }
            }

            return chosen;
        }

        /// <summary>The name of the class Make makes a default of: the class itself, or the one registered with the type id.</summary>
        private static string ClassNameOf(object key)
        {
            if (key is Type kind)
            {
                return kind.Name;
            }

            foreach (var registered in ids)
            {
                if (registered.Value == (int)key)
                {
                    return registered.Key.Name;
                }
            }

            return key.ToString()!;
        }

        /// <summary>The default object of a unit type, made once: by the class registered for the type, else, when a class registered with the id asks before its maker is registered (a setup of the map's own; Registering registers every maker first), as that class with its own constructor (ClassOfId), else as a Hero for a hero type and a Unit for any other (as KnownUnits.TakeIn makes a unit), with the type id, no handle, and gone. </summary>
        private static Unit DefaultUnit(int typeId, bool hero, Type? asked)
        {
            if (unitDefaults == null)
            {
                unitDefaults = new Dictionary<int, Unit>();
            }

            if (unitDefaults.TryGetValue(typeId, out var known))
            {
                return known;
            }

            var maker = UnitMaker(typeId);
            var made = maker != null ? Make(typeId, maker)
                : asked != null ? Make(typeId, () => (Unit)Another(ClassOfId(typeId, asked)))
                : KnownUnits.Plain(hero);
            made.TypeId = typeId;
            made.IsGone = true;
            unitDefaults[typeId] = made;
            return made;
        }

        /// <summary>The default object of an item type, made once: by the class registered for the type, else as the class that asks before its maker is registered (as DefaultUnit), else as an Item, with the type id, no handle, and gone. </summary>
        private static Item DefaultItem(int typeId, Type? asked)
        {
            if (itemDefaults == null)
            {
                itemDefaults = new Dictionary<int, Item>();
            }

            if (itemDefaults.TryGetValue(typeId, out var known))
            {
                return known;
            }

            var maker = ItemMaker(typeId);
            var made = maker != null ? Make(typeId, maker)
                : asked != null ? Make(typeId, () => (Item)Another(ClassOfId(typeId, asked)))
                : new Item();
            made.TypeId = typeId;
            made.IsGone = true;
            itemDefaults[typeId] = made;
            return made;
        }

        /// <summary>The object id made from four characters, the first one the highest byte.</summary>
        [LocalSafe]
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

        /// <summary>The function that makes the object of a registered item type; null for an id that is not registered.</summary>
        internal static Func<Item>? ItemMaker(int id)
        {
            if (itemMakers.TryGetValue(id, out var maker))
            {
                return maker;
            }

            return null;
        }

        /// <summary>The function that makes the object of a registered destructable type; null for an id that is not registered.</summary>
        internal static Func<Destructable>? DestructableMaker(int id)
        {
            if (destructableMakers.TryGetValue(id, out var maker))
            {
                return maker;
            }

            return null;
        }

        /// <summary>The function that makes the object of a registered ability type; null for an id that is not registered.</summary>
        internal static Func<Ability>? AbilityMaker(int id)
        {
            if (abilityMakers.TryGetValue(id, out var maker))
            {
                return maker;
            }

            return null;
        }

        /// <summary>The hidden ability that carries a buff on its unit: 0 when its class has no display.</summary>
        internal static int CarrierOf(Buff buff)
        {
            if (carriers.TryGetValue(Names.ClassOf(buff), out var carrier))
            {
                return carrier;
            }

            return 0;
        }

        /// <summary>The game buff a buff shows: 0 when its class has no display.</summary>
        internal static int BuffIdOf(Buff buff)
        {
            if (buffIds.TryGetValue(Names.ClassOf(buff), out var id))
            {
                return id;
            }

            return 0;
        }

        /// <summary>Forgets every registered type; Runtime.Reset calls it.</summary>
        internal static void Reset()
        {
            makers = new Dictionary<int, Func<Unit>>();
            itemMakers = new Dictionary<int, Func<Item>>();
            destructableMakers = new Dictionary<int, Func<Destructable>>();
            abilityMakers = new Dictionary<int, Func<Moonwell.Ability>>();
            ids = new Dictionary<Type, int>();
            buffIds = new Dictionary<object, int>();
            carriers = new Dictionary<object, int>();
            unitDefaults = null;
            itemDefaults = null;
            classDefaults = null;
            making = null;
            waiting = null;
        }

        /// <summary>The character code at an index of a string.</summary>
        private static int Code(string code, int index)
        {
            return (int)code[index];
        }
    }
}
