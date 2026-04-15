# Tasks — Send Core Rework

## 1. Arquitectura del core

1.1 Definir los use cases explícitos del envío y sus structs de entrada/salida.  
1.2 Separar la orquestación fuera del monolito actual de `internal/service/send.go`.  
1.3 Delimitar puertos nuevos o refinados para batch suppression, hot/cold storage y rate limiting.  
1.4 Alinear el worker con la nueva frontera de persistencia.

## 2. Base de datos y persistencia

2.1 Diseñar el split hot/cold de `emails` y crear las migraciones necesarias.  
2.2 Implementar la consulta batch para supresión global + workspace.  
2.3 Rediseñar el camino de rate limiting para reducir contención por adapter.  
2.4 Adaptar `email_repo` y los repositorios relacionados al nuevo modelo.  
2.5 Ajustar índices y queries de lectura para la nueva distribución de datos.

## 3. Pipeline de envío

3.1 Reescribir la resolución del contexto de envío como use case independiente.  
3.2 Reemplazar la supresión secuencial por evaluación batched.  
3.3 Persistir fila caliente y payload frío con una transacción coherente.  
3.4 Mantener el contrato funcional del enqueue y de los tracking IDs.  
3.5 Ajustar el worker para cargar el payload frío solo cuando lo necesite.

## 4. Verificación, E2E y bench

4.1 Rehacer/actualizar los tests unitarios de los nuevos use cases.  
4.2 Agregar tests de integración para batch suppression, persistencia y rate limiting.  
4.3 Ejecutar validación E2E autónoma contra el flujo reescrito.  
4.4 Correr el bench funcional del hot path y comparar contra el baseline.  
4.5 Consolidar el signoff técnico final con Volta + Kuhn.
