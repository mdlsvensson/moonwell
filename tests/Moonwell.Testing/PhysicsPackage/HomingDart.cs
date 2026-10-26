namespace Moonwell.Testing
{
    /// <summary>A missile the component cases fire, flying through the ground, that lists a Homing in its constructor, as a map's seeker does.</summary>
    public sealed class HomingDart : Missile
    {
        /// <summary>Makes a dart that flies through the ground with a Homing of a turn rate and no target.</summary>
        public HomingDart(float turnRate)
        {
            GroundContact = Contact.None;
            Components.Add(new Homing { TurnRate = turnRate });
        }
    }
}
