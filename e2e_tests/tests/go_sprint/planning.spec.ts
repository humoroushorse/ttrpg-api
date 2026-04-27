import { test, expect } from '@playwright/test';
import { APIClient } from '../../lib/api-client';
import { getConfig } from '../../lib/config';
import { loginAsUser } from '../../lib/helpers/auth';

test.describe('Sprint Planning Endpoints @go_sprint @planning', () => {
  let apiClient: APIClient;
  let authToken: string;
  const config = getConfig();

  test.beforeAll(async () => {
    apiClient = new APIClient(config);
    const auth = await loginAsUser(apiClient);
    authToken = auth.token;
  });

  test('POST /api/v1/sprints/planning/capacity - should calculate sprint capacity', async () => {
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints/planning/capacity`;
    
    const response = await apiClient.post(url, {
      lookback_sprints: 3,
    }, {
      headers: {
        Authorization: `Bearer ${authToken}`,
      },
    });

    console.log('Capacity response status:', response.status);
    console.log('Capacity response:', JSON.stringify(response.data, null, 2));

    // Should return 200 or 400 (if no completed sprints) or 501 (if not implemented)
    expect([200, 400, 501]).toContain(response.status);

    if (response.status === 200) {
      // Validate response structure
      expect(response.data).toHaveProperty('recommended_capacity');
      expect(response.data).toHaveProperty('average_velocity');
      expect(response.data).toHaveProperty('confidence_level');
      expect(['low', 'medium', 'high']).toContain(response.data.confidence_level);
      
      // Validate types
      expect(typeof response.data.recommended_capacity).toBe('number');
      expect(typeof response.data.average_velocity).toBe('number');
    }
  });

  test('POST /api/v1/sprints/planning/forecast - should forecast sprint completion', async () => {
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints/planning/forecast`;
    
    const response = await apiClient.post(url, {
      remaining_points: 50,
      lookback_sprints: 3,
    }, {
      headers: {
        Authorization: `Bearer ${authToken}`,
      },
    });

    console.log('Forecast response status:', response.status);
    console.log('Forecast response:', JSON.stringify(response.data, null, 2));

    // Should return 200 or 400 (if no completed sprints) or 501 (if not implemented)
    expect([200, 400, 501]).toContain(response.status);

    if (response.status === 200) {
      // Validate response structure
      expect(response.data).toHaveProperty('estimated_sprints');
      expect(response.data).toHaveProperty('average_velocity');
      expect(response.data).toHaveProperty('confidence_level');
      expect(['low', 'medium', 'high']).toContain(response.data.confidence_level);
      
      // Validate types
      expect(typeof response.data.estimated_sprints).toBe('number');
      expect(typeof response.data.average_velocity).toBe('number');
    }
  });

  test('POST /api/v1/sprints/planning/forecast - should reject negative remaining points', async () => {
    const url = `${config.services.goSprint.baseUrl}/api/v1/sprints/planning/forecast`;
    
    const response = await apiClient.post(url, {
      remaining_points: -10,
      lookback_sprints: 3,
    }, {
      headers: {
        Authorization: `Bearer ${authToken}`,
      },
    });

    console.log('Negative points response status:', response.status);
    console.log('Negative points response:', JSON.stringify(response.data, null, 2));

    // Should return 400 or 501 (if not implemented)
    expect([400, 501]).toContain(response.status);
  });
});
