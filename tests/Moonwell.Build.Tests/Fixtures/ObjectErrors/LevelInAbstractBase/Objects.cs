using Moonwell;

namespace LevelInAbstractBase
{
    public sealed class LevelInAbstractBaseMap : Map
    {
    }

    // An abstract base and a generic one declare what the classes that inherit them start with.
    public abstract class Trained : Moonwell.Objects.Human.Units.Footman
    {
        protected Trained()
        {
            Level = 3;
        }
    }

    public class Squad<T> : Moonwell.Objects.Human.Units.Footman
    {
        public Squad()
        {
            Level = 4;
        }
    }

    public class Veteran : Trained
    {
    }

    public class Sergeant : Squad<Veteran>
    {
    }
}
