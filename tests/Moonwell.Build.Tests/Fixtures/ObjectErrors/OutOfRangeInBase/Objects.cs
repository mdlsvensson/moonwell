using Moonwell;

namespace OutOfRangeInBase
{
    public sealed class OutOfRangeInBaseMap : Map
    {
    }

    // The value is put in the base's constructor, which runs for the class that inherits it.
    public abstract class Sentry : Moonwell.Objects.Human.Units.Footman
    {
        protected Sentry()
        {
            MaxLife = 600000;
        }
    }

    public class Drillmaster : Sentry
    {
        public Drillmaster()
        {
            Name = "Drillmaster";
        }
    }
}
